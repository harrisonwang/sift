# sift

provider 驱动的技术资讯雷达。`sift` 从配置的数据源（Twitter/X via Nitter、Hacker
News、Reddit、RSS/Atom 博客）抓取条目，用 SQLite 缓存去重，并生成 Markdown/JSON
报告——全部由一个零运行时依赖的静态单二进制完成。

## 设计原则

- **单一二进制、零运行时依赖** —— 纯 Go 静态编译（`modernc.org/sqlite`，无 CGO）。
- **provider 驱动** —— 每个数据源都是一个 `Provider`，新增源不动核心。
- **CLI 是唯一入口** —— 可脚本化、可 cron、可被 agent 调用。
- **标准化数据模型** —— 所有源都归一为同一个 `Item`，存储与报告同数据源解耦。
- **配置与代码分离** —— 用 YAML 激活并参数化各 provider。

## 架构

```
┌──────────────────────────────────────────┐
│                  CLI 层                    │
│   discover · query · report · prune · ... │
├──────────────────────────────────────────┤
│             orchestration 编排层           │
│        调度 · 去重 · 报告生成              │
├──────────┬──────────┬──────────┬──────────┤
│ Twitter  │   HN     │ Reddit   │ rssblog  │
│ Provider │ Provider │ Provider │ Provider │
├──────────┴──────────┴──────────┴──────────┤
│               Cache（SQLite）              │
└──────────────────────────────────────────┘
```

编排层并发抓取所有启用的 provider，按 `(provider, external_id)` 经缓存去重，再把
标准化后的条目交给 reporter。完整设计见 `docs/plan.md`。

## 安装 / 构建

macOS 可通过 Homebrew tap 安装：

```bash
brew tap harrisonwang/tap
brew install sift
```

Windows 可通过 Scoop bucket 安装：

```powershell
scoop bucket add harrisonwang https://github.com/harrisonwang/scoop-bucket
scoop install harrisonwang/sift
```

从源码构建需要 Go 1.25+。

```bash
make build          # -> ./bin/sift   （关闭 CGO，静态编译）
# 或
go build -o bin/sift ./cmd/sift
```

纯 Go，开箱即可交叉编译：

```bash
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o sift-linux  ./cmd/sift
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o sift.exe    ./cmd/sift
```

## 快速开始

```bash
sift config init                                  # 创建 $HOME/.sift/config.yaml（已存在则不覆盖）
sift add @karpathy https://simonwillison.net/ --write   # 一键加源：识别+验证+写配置（推荐）
# 或手动编辑 $HOME/.sift/config.yaml 调整 provider
sift config validate                              # 校验配置与各 provider 设置
sift discover                                     # 抓取新条目入缓存；打印 JSON 摘要
sift query --date today                           # 今天的条目，JSON 输出到 stdout
sift report --date today                          # 今天的简报（Markdown）输出到 stdout
sift report --date today -o digest.md             # ……或写入文件
```

如需覆盖重建配置文件，可运行 `sift config init --force`。源码目录中的
`config.example.yaml` 与 `sift config init` 写入的 starter config 保持一致。

## 命令

| 命令 | 用途 |
|---|---|
| `sift add <input>` | 识别信息源并加入雷达（`@handle` / `r/sub` / 网站 / GitHub repo）；默认 dry-run，`--write` 写配置。 |
| `sift discover` | 抓取所有启用 provider 的新条目入缓存；向 stdout 打印 JSON 摘要。 |
| `sift query` | 查询缓存并输出到 stdout（默认 JSON）。 |
| `sift report` | 从缓存生成报告（默认 Markdown 到 stdout；`-o` 写入文件）。 |
| `sift prune` | 按过滤条件删除缓存条目（改了 provider 过滤后用它清理旧数据）。 |
| `sift config init` | 创建默认配置文件 `$HOME/.sift/config.yaml`；已存在则不覆盖，可加 `--force` 覆盖。 |
| `sift config validate` | 校验配置，含各 provider 的设置。 |
| `sift config show` | 打印当前生效的配置。 |
| `sift source list` | 列出可用的 provider 类型。 |
| `sift source info <name>` | 查看某 provider 的配置项与示例。 |

全局参数：`-c/--config <path>`（默认 `$HOME/.sift/config.yaml`）、`--db <path>`（覆盖缓存位置，默认 `$HOME/.sift/sift.db`）、
`-v/--verbose`（调试日志）、`-q/--quiet`（静默 info/warn 日志）。所有命令结果走
**stdout**，日志走 **stderr**。

### 过滤（`query` 与 `report`）

```bash
sift query  --provider hackernews --limit 20
sift query  --source twitter:msdev --format json
sift report --date 2026-06-03 --keyword "release" -o out.md
sift report --format json -o report.json
```

- `--date` 接受 `YYYY-MM-DD`、`today`、`yesterday`（按条目**首次入缓存的本地日期**过滤）。
- `--keyword` 对标题 + 摘要做大小写不敏感的子串匹配。
- `discover`/`query`/`report` 都写 **stdout**，日志走 **stderr**，所以管道很干净：
  `sift query --date today | jq '.items[].url'`。加 `--quiet` 可彻底静默日志。

## 添加信息源（`sift add`）

不用手写 provider schema——直接把你知道的东西丢给 `sift add`，它会
**识别类型 → 抓一次验证 → 写入配置**：

```bash
sift add @karpathy                     # Twitter/X 账号
sift add r/LocalLLaMA                   # Reddit 子版块
sift add https://simonwillison.net/    # 普通网站（自动发现 RSS/Atom）
sift add github.com/cli/cli            # GitHub release feed
sift add hn                            # Hacker News
sift add @karpathy r/golang --write    # 批量；--write 才写入配置
```

- 默认 **dry-run**：只识别 + 验证，输出 JSON（给 Agent）或加 `-f text`（给人），不改配置。
- `--write` 才合并进 `$HOME/.sift/config.yaml`：幂等去重，保留已有注释。
- **非交互**：一个网站发现多个 feed 时，默认选第一个，其余放进 `candidates`；识别 / 验证
  失败以非零退出码返回。
- `--no-verify` 跳过抓取验证（离线 / CI）。

## 配置

默认配置文件为 `$HOME/.sift/config.yaml`，默认 SQLite 缓存为 `$HOME/.sift/sift.db`。
`cache.db_path` 可省略；只有需要自定义缓存位置时才配置。

```yaml
cache: {}
providers:
  - name: twitter
    enabled: true
    config:
      accounts: [msdev, OpenAI]
      nitter_instances: [https://nitter.net, https://nitter.privacydev.net]
      max_items: 20
      include_retweets: false   # 默认：丢弃转推
      include_replies: false    # 是否保留账号自己的回复
  - name: hackernews
    enabled: true
    config:
      feeds: [top, show]   # top | new | show | ask | best
      max_items: 50
  - name: reddit
    enabled: true
    config:
      subreddits: [programming, MachineLearning]
      sort: hot            # hot | new | top | rising | best
      limit: 30
  - name: rssblog
    enabled: true
    config:
      feeds:
        - url: https://openai.com/news/rss.xml
          source: openai_news
        - url: https://blog.google/products/gemini/rss/
          source: gemini_blog
        - url: https://deepmind.google/blog/rss.xml
          source: deepmind_blog
      max_items: 30
```

各 provider 的确切配置项可用 `sift source info <name>` 查看。

## Provider

| Provider | 数据源 | 说明 |
|---|---|---|
| `twitter` | Twitter/X 账号 via Nitter RSS | 多实例 fallback；统一为 `x.com` 链接；可过滤转推/回复（见下）。 |
| `hackernews` | HN Firebase API | 并发、有上限地抓取条目。 |
| `reddit` | 子版块 RSS | `sort` 可选。 |
| `rssblog` | 任意 RSS/Atom 源 | 博客、release notes、论文 feed 等。注：Anthropic 无官方 RSS。 |

> **关于 twitter 噪音的调优。** 一个 Nitter 账号 feed 是该账号的**完整时间线**——
> 原创推文 **加上转推**（`RT by @acct:`）**和回复**（`R to @x:`）。`include_retweets`
> （默认 `false`）与 `include_replies`（默认 `true`）控制保留哪些。由于过滤在
> `discover` 时生效，已经用更宽松设置入库的旧数据不会自动消失——**收紧过滤后**，
> 先 `sift prune --source twitter:<账号>`（或 `--provider twitter`）再 `sift discover`。

## 定时 / 触发

**cron**（每天 08:00）：

```cron
0 8 * * * cd /path/to/sift && ./bin/sift -q discover && ./bin/sift -q report --date today -o reports/$(date +\%F).md
```

**GitHub Actions**：

```yaml
on:
  schedule:
    - cron: "0 0 * * *"
jobs:
  radar:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.25" }
      - run: go build -o sift ./cmd/sift
      - run: ./sift -q discover
      - run: ./sift -q report --date today -o report.md
      - uses: actions/upload-artifact@v4
        with: { name: report, path: report.md }
```

### 在 agent / automation 中使用

任何能执行 shell 命令并读取 stdout 的工具都能直接驱动 `sift`，无需中间文件：

```bash
sift -q discover && sift -q query --date today -f markdown   # 把 stdout 喂给模型
sift -q query --date today | jq '.items'                     # 或拿结构化 JSON
```

- **WorkBuddy Automation / cron / CI**：执行上面的命令，读 stdout 即可。
- **Claude Code**：在 `.claude/settings.json` 里放行命令，例如
  `"Bash(sift discover)"`、`"Bash(sift query:*)"`、`"Bash(sift report:*)"`。
- **不能跑 shell 的沙箱 agent**（Claude Desktop、ChatGPT Desktop）：需要一个 MCP
  server（`sift mcp`），这是预留的后续扩展（见 `docs/plan.md` §8），尚未实现。

## 开发

```bash
make test      # go test ./...
make vet       # go vet ./...
make fmt       # gofmt -s -w .
```

新增一个 provider：在 `internal/provider/<name>/` 实现 `provider.Provider`，在
`init()` 里用 `provider.Register(Info(), New)` 自注册，并把它的 blank import 加到
`internal/provider/all`。无需改动数据模型、缓存或编排层。
```
