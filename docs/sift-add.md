# `sift add` — 信息源解析器（设计）

> **状态:MVP 已实现**(`internal/resolver` + `internal/config/source.go` +
> `cmd/sift/add.go`)。第一批识别、RSS 自动发现、验证、`--write` 幂等合并、JSON/text
> 输出均已落地;第二批识别(YouTube/Mastodon)、preset 等仍为后续。

## 1. 要解决的问题

用户心里只有一个**信息对象**:一个 URL、一个 handle、一个网站、一个 subreddit。
但今天的 sift 要求先理解 provider schema——知道 `@karpathy` 要写进 `twitter.accounts`、
`r/LocalLLaMA` 要写进 `reddit.subreddits`、某个博客得先翻出它的 RSS 地址塞进
`rssblog.feeds`。

**这层"从信息对象到配置 schema"的翻译,就是最大的摩擦。**

结论先行:

> `sift add` 应该成为用户和 Agent 接入信息源的**主入口**;模板 / preset 只是冷启动辅助,
> 不应该是主路径。

## 2. 它是什么,不是什么

`sift add` 的本质**不是**"给我一个网站,我无限深度爬",而是一个**信息源解析器
(source resolver)**:

```
输入用户知道的东西
  → 识别信息源类型
  → 验证能抓到数据
  → （可选）写入配置
```

它不是 crawler,不做深度抓取,也不维护领域知识。

## 3. 调用模型:Agent 调用,人通过聊天下指令（最重要的前提）

**这个 CLI 的真正使用者是 Agent**(WorkBuddy、Hermes Agents、OpenClaw 这类运行时),
人是在聊天里下达指令、由 Agent 翻译成 CLI 调用。这个前提决定了几条硬约束:

1. **首要产出是给 Agent 的结构化数据,不是给人看的文本。** 人不会直接看 CLI 输出,看到的是
   Agent 在聊天里的转述。所以 `sift add` **默认输出 JSON**(和 `discover`/`query` 一致);
   "好看的文本版"由 Agent 读 JSON 后自己在聊天里组织,CLI 只在 `--format text` 时才给人看。
2. **自然语言理解留在 Agent / 聊天层。** CLI 只接受**具体输入**(URL / handle / `r/x`),
   不解析自然语言。"帮我关注 Karpathy" 这种话由 Agent 抽取成 `@karpathy` 再调用——CLI 保持
   确定、可测。
3. **完全非交互,绝不阻塞 stdin。** 不能有 `y/n` 提示、不能交互式选择。遇到歧义(比如一个站点
   发现多个 feed),**返回候选**让调用方决定,而不是卡住等输入。确认这件事发生在**聊天层**
   (Agent 问人),不在 CLI 里。
4. **退出码 + 结构化字段 = Agent 的分支信号。** Agent 靠退出码和 JSON 字段判断"成功 / 没识别
   出 / 验证失败 / 有歧义",不靠解析人类文本。
5. **JSON 输出是公共契约。** 多个 Agent 依赖它,所以**只做加法、字段稳定、版本化**。

## 4. 命令形态

```bash
sift add <input>...                 # 默认:识别 + 验证，输出 JSON 结果（不写配置）
sift add <input>... --write         # 识别 + 验证通过后，写入 ~/.sift/config.yaml
sift add <input>... --format text   # 给人看的文本（调试用）
sift add <input>... --no-verify     # 跳过抓取验证（离线 / CI）
```

- **默认 dry-run、默认 JSON**:既安全(不乱改配置),又正好喂给 Agent。
- 主推 `sift add`;`sift source add` 作为别名保留(更规整,但 `sift add` 更像主入口)。
- 第一版就支持**多个 input**:各自独立识别和验证,结果汇总成数组。

## 5. 核心三步:识别 → 验证 → 落配置

### 5.1 识别(resolver)

**判定顺序**必须"特化在前、通用兜底在后",否则 `x.com/karpathy` 会先被当成普通网站去做
RSS 发现:

1. `@handle` 形式 → twitter
2. **已知 host**:`x.com` / `twitter.com` / `nitter.*` → twitter;`reddit.com/r/…` 或
   `r/…` → reddit;`news.ycombinator.com` / `hn` → hackernews;`github.com/owner/repo`
   → rssblog(`…/releases.atom`)
3. **feed 形状**:URL 以 `.rss` / `.atom` / `.xml` 结尾,或含 `/feed`、`/rss` → rssblog
4. **通用网站** → **RSS/Atom 自动发现**(见下)
5. 都不中 → 识别失败,在 JSON 里给出原因和手动指引

> ⚠️ 注意区分:上面是**解析判定顺序**(特化→通用);§7 的"第一批 / 第二批"是**构建优先级**
> (先实现哪些)。两者不是一回事。

**RSS/Atom 自动发现**(覆盖长尾网站的关键):抓取页面 → 解析 `<head>` 里的
`<link rel="alternate" type="application/rss+xml|atom+xml" href="…">`(相对地址转绝对)。
发现不到再探测常见路径兜底:`/feed`、`/rss`、`/index.xml`、`/atom.xml`;仍找不到才算失败。

> 实现成本低:HTML 解析用 `goquery`(gofeed 已带进来),抓取 / 解析 feed 复用现有 `feedx`。

**多个 feed 的歧义(非交互处理):** 自动发现到多个 feed 时,**默认选第一个**作为
`feed_url`,并把全部候选放进 JSON 的 `candidates`。Agent 想换一个,直接用那个**显式 feed
URL** 再调一次 `sift add`(命中第 3 步"feed 形状")即可——不需要交互、也不需要额外的选择参数。

### 5.2 验证(核心,不能省)

- 识别后**立即抓一次**,把条数和最新一条标题放进结果。这才是"配置即用"的确定感,也是 Agent
  能向人确认"抓到了 12 条"的依据。
- 验证失败时**默认不写**配置;`--no-verify` 可显式跳过(离线 / CI)。
- **twitter 验证要宽容**:它依赖 Nitter,失败往往是某个实例挂了,而不是 handle 写错。要多实例
  重试;仍失败时在结果里标注"可能是 Nitter 实例问题,而非账号无效",不武断判错。

### 5.3 落配置(`--write`)

- **合并进已有 provider 块**:twitter 追加 `accounts`、reddit 追加 `subreddits`、rssblog 追加
  `feeds`;对应块不存在则新建。
- **幂等**:重复调用不重复追加。去重规则——twitter 按 handle、reddit 按 subreddit 小写、
  rssblog 按 feed URL。已存在则在结果里标 `duplicate: true`。
- **写回实现**:用 `yaml.Node` 做**树级编辑**(把新节点挂到对应序列下),能**保住已有节点的
  注释**,只有整体格式可能被规范化。
- **无配置文件时**:dry-run 照常返回结果;`--write` 找不到配置时,在 JSON 里返回明确错误——
  提示先 `sift config init`(第一版不自动建,保持行为可预期)。

## 6. 输出契约

### 6.1 默认:JSON(给 Agent)

`results` 永远是数组(单个 input 也是单元素数组),便于 Agent 统一解析:

```json
{
  "results": [
    {
      "input": "@karpathy",
      "resolved": true,
      "provider": "twitter",
      "key": "karpathy",
      "verified": true,
      "sample_count": 12,
      "latest_title": "...",
      "ambiguous": false,
      "candidates": [],
      "written": false,
      "duplicate": false,
      "suggestion": "providers:\n  - name: twitter\n    enabled: true\n    config:\n      accounts:\n        - karpathy\n",
      "error": ""
    }
  ]
}
```

- rssblog 类结果额外带 `feed_url` 和建议的 `source` 名。
- 字段语义稳定;新增字段只做加法,不改已有字段含义。

**退出码**:全部 input 都识别且验证成功(`--write` 时还要写入成功)→ `0`;只要有一个失败 →
非零。Agent 先看退出码,再按 `results[].resolved` / `verified` 取细节。

### 6.2 `--format text`:给人(调试)

```text
$ sift add https://simonwillison.net/ --format text
✓ 识别为网站:https://simonwillison.net/
✓ 自动发现 feed:https://simonwillison.net/atom/everything/
✓ 验证成功:获取到 20 条内容（最新:...）

建议配置片段:
  providers:
    - name: rssblog
      enabled: true
      config:
        feeds:
          - url: https://simonwillison.net/atom/everything/
            source: simonwillison

写入配置请加 --write
```

### 6.3 Agent 在聊天里转述给人的样子(示意,非 CLI 输出)

> 已识别为 Simon Willison 的博客,自动找到 RSS 并验证成功(抓到 20 条,最新《…》)。要我加入
> 你的关注列表吗?

— 这是 Agent 读 JSON 后自己组织的话术,CLI 不负责。

## 7. 识别优先级(构建顺序)

### 第一批(MVP 必做)

| 输入 | 识别为 |
|---|---|
| `@karpathy` | twitter account |
| `x.com/karpathy` / `twitter.com/karpathy` | twitter account |
| `r/LocalLLaMA` / `reddit.com/r/LocalLLaMA` | reddit subreddit |
| `news.ycombinator.com` / `hn` | hackernews |
| 以 `.rss` / `.atom` / `.xml` 结尾或含 `/feed`、`/rss` 的 URL | rssblog feed |
| 普通网站 URL | RSS/Atom 自动发现 → rssblog |
| `github.com/owner/repo` | rssblog:`releases.atom` |

### 第二批(之后再做)

YouTube 频道(需解析 channel_id)、GitHub commits/tags/issues、Mastodon、Medium/Substack
(RSS 自动发现兜底即可)。

## 8. 边界与取舍

- **不做通用爬虫**,不做深度抓取;只识别"是不是一个可跟踪的信息源"。
- **不做自然语言解析**:那是 Agent / 聊天层的事,CLI 只吃具体 input。
- **完全非交互**:歧义靠返回 `candidates`,确认靠聊天层。
- **写回会规范化 YAML**:树级编辑能保住已有注释,但整体格式可能被标准化;在意格式就只用
  dry-run 自己粘贴。
- **验证需要网络**:`--no-verify` 作为离线 / CI 兜底。

## 9. MVP 范围

**做:** `sift add <input>...`(多个)· 第一批识别 + RSS/Atom 自动发现 · 抓取验证(默认开,
`--no-verify` 跳过)· 默认 dry-run + JSON 输出 + 非零退出码 · `--write` 合并写入且幂等 ·
`--format text` 人类视图 · 无配置时 dry-run 可用、`--write` 给明确错误 · 多 feed 歧义返回
candidates。

**不做(留后):** 第二批识别 · preset / 主题包 · `--write --init` 自动建配置 ·
`--allow-invalid` 等高级开关。

## 10. 与模板 / preset 的关系

- `sift add` 解决**高频核心摩擦**:"我想关注一个新东西,但不想研究配置 schema。"
- preset 解决**冷启动**:"我连关注什么都不知道,只知道主题。"——更难维护,需要持续投入领域
  知识。

优先级:① `sift add`(本文档)→ ② `sift config init` 的模板选项 → ③ 以后再做 preset。
不要把 preset 混进 `add` 的 MVP。

## 11. 与现有架构的契合

- **复用**:`feedx`(抓取 / 解析)、`goquery`(HTML 解析,已是依赖)、provider 配置结构、
  `config.Load` / `config.InitDefaultConfig`。
- **不动核心**:解析器是 provider 之上的一层,不改 `Item`、cache、orchestrator。
- **人和 Agent 共用一个入口**:Agent 用默认 JSON、人用 `--format text`,同一条命令;`sift add`
  就是"接入信息源"的统一 API。
- **契约稳定**:`Item` schema、各命令的 JSON 输出(含 `sift add` 的 `results`)都是 Agent 依赖
  的公共契约,只做加法、版本化。
