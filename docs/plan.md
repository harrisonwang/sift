## 1. 项目名称：`sift`

## 2. 设计原则

- **单一二进制，零运行时依赖**
- **Provider 驱动，无限扩展**
- **CLI 唯一入口** — 所有交互通过命令行完成
- **标准化数据模型** — 所有源输出统一条目，报告层与源解耦
- **配置与代码分离** — YAML 配置激活和参数化 Provider
- **纯 Go 静态编译** — `modernc.org/sqlite`

---

## 3. 架构

```
┌──────────────────────────────────────────┐
│                 CLI 层                    │
│  add · discover · query · report · prune  │
│       config · source (cobra 命令树)      │
├──────────────────────────────────────────┤
│             orchestration 层              │
│     调度 · 去重 · 报告生成 · 配置管理     │
├──────────┬──────────┬──────────┬─────────┤
│ Twitter  │   HN     │ Reddit   │  Blog   │
│ Provider │ Provider │ Provider │ Provider│
├──────────┴──────────┴──────────┴─────────┤
│              Cache (SQLite)               │
└──────────────────────────────────────────┘
```

- **CLI 层**：面向用户/脚本/Agent 的唯一接口
- **Orchestration 层**：串联 Provider 调用、去重、报告生成的业务逻辑
- **Provider 层**：数据源抽象
- **Cache 层**：持久化

如果未来需要 MCP/SKILL，可以在 CLI 层之上包装一个轻量服务，但现在不实现、不维护。

---

## 4. 目录结构

```
sift/
├── cmd/
│   └── sift/                          # cobra 命令树
│       ├── main.go  root.go           # 入口 + 根命令/全局参数（含版本、--quiet）
│       ├── add.go                     # sift add（信息源解析器入口）
│       ├── discover.go  query.go  report.go  prune.go
│       └── configcmd.go  source.go  filter.go
├── internal/
│   ├── model/item.go                  # Item 结构体
│   ├── config/
│   │   ├── config.go                  # YAML 解析（默认 ~/.sift）
│   │   ├── source.go                  # AddSource：yaml.Node 合并写配置
│   │   └── default_config.go          # config init 的内嵌 starter
│   ├── resolver/resolver.go           # 识别 + RSS 自动发现 + 验证（sift add 引擎）
│   ├── feedx/feedx.go                 # 共享 RSS 抓取/解析工具
│   ├── provider/
│   │   ├── provider.go  registry.go   # 接口 + 自注册式工厂
│   │   ├── all/all.go                 # blank import 触发注册
│   │   └── twitter/  hackernews/  reddit/  rssblog/
│   ├── cache/
│   │   ├── cache.go                   # Cache 接口
│   │   └── sqlite.go                  # SQLite 实现（modernc，无 CGO）
│   ├── reporter/
│   │   └── reporter.go  markdown.go  json.go
│   └── orchestrator/orchestrator.go   # 并发抓取 → 去重 → 查询/报告
├── config.example.yaml
├── Makefile  go.mod  go.sum  .goreleaser.yaml  .gitignore
├── .github/workflows/                 # ci.yml + release.yml
└── README.md
```

MCP 若未来需要（仅当目标 Agent 跑不了 shell），可另起 `internal/server/` 作为独立包，
不影响核心；详见 `agent-integration.md`。

---

## 5. CLI 命令设计

```bash
# 核心命令
sift add @karpathy                    # 识别信息源并加入雷达（keystone）
sift discover                         # 抓取今天发布的新内容写入缓存（JSON 摘要到 stdout）
sift query --date 2026-06-03          # 从缓存查询（默认 JSON 到 stdout）
sift report --date 2026-06-03         # 生成报告（默认 Markdown 到 stdout，-o 写文件）

# 辅助命令
sift prune --provider twitter         # 按条件清理缓存
sift config init|validate|show        # 配置管理
sift source list|info <name>          # 查看 Provider 信息
```

- `add`：把一个 URL / handle / subreddit / 网站识别为信息源并写入配置（keystone）
- `discover`：遍历所有启用的 Provider，抓取今天发布的新内容，写入缓存
- `query`：从缓存按条件查询，默认输出 JSON 到 stdout
- `report`：生成报告，默认 Markdown 到 stdout（`-o` 写文件）
- `prune`：按条件清理缓存
- `config`：配置相关的管理命令（含 `init`）
- `source`：查看 Provider 信息

---

## 6. 配置设计

默认配置文件 `~/.sift/config.yaml`（`sift config init` 生成），默认缓存 `~/.sift/sift.db`。
大多数源不必手写，用 `sift add <url|@handle|r/sub|网站>` 自动识别并写入。

```yaml
cache: {}              # db_path 可省略；省略即用 ~/.sift/sift.db

providers:
  - name: twitter
    enabled: true
    config:
      accounts: [msdev, OpenAI]
      nitter_instances:
        - https://nitter.net
        - https://nitter.privacydev.net
      include_retweets: false   # 默认丢弃转推
      include_replies: false    # 是否保留账号自己的回复

  - name: hackernews
    enabled: true
    config:
      feeds: [top, show]
      max_items: 50

  - name: reddit
    enabled: true
    config:
      subreddits: [programming, MachineLearning]
      sort: hot
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
        # 注：Anthropic 无官方 RSS
      max_items: 30
```

---

## 7. 开发路线图

> 阶段 1–6 已全部完成；阶段 7 为方案演进后新增。

### 阶段 1：项目骨架 + 核心模型
- 初始化 Go module
- 定义 `model.Item`
- 定义 `Provider` 接口
- 定义 `Cache` 接口
- 搭建 `cobra` 命令树（空命令占位）
- 可编译出 `sift` 二进制，但命令都是空的

### 阶段 2：配置 + Cache + 通用 RSS Provider
- `config` 包：解析 YAML
- `cache/sqlite.go`：SQLite 建表 + `InsertIfNew` + 简单查询
- `rssblog` Provider：实现通用 RSS 抓取（用 `gofeed` 库）
- `orchestrator.discover`：遍历 Provider → 调用 Fetch → 写缓存
- 此时可以用 `discover` 命令抓取 OpenAI/Anthropic 博客

### 阶段 3：Twitter + HN Provider
- `twitter` Provider：Nitter RSS 抓取 + 多实例 fallback
- `hackernews` Provider：Firebase API 抓取 top/new/show
- 此时已能覆盖 Twitter、HN、博客三类源

### 阶段 4：查询 + 报告
- `query` 命令：按日期/源/关键词从缓存查询，输出 JSON
- `report` 命令：Markdown/JSON 格式报告
- 此时 `discover → query → report` 完整链路跑通

### 阶段 5：Reddit Provider + 生产加固
- `reddit` Provider：子版块 RSS 抓取
- 并发优化（goroutine + errgroup 并发抓取多源）
- 超时、重试、错误处理
- 日志

### 阶段 6：CI/CD + 文档
- `.goreleaser.yaml` 多平台编译
- GitHub Actions 自动 release
- README + 各平台触发示例（cron、WorkBuddy Automation、GitHub Actions）

### 阶段 7：Agent 接入 + 易用性
- `sift add` 信息源解析器：识别 + RSS 自动发现 + 验证 + 幂等写配置（keystone）
- `~/.sift` 用户级布局 + `sift config init` 内嵌 starter；`prune` 缓存清理
- 输出面向 Agent 友好化：`discover` 出 JSON、`report` 默认 stdout、`--quiet`
- 全中文 CLI 帮助与文档；Homebrew tap / Scoop bucket 分发
- 控制面 / 数据面策略见 `agent-integration.md`、`sift-add.md`

---

## 8. 未来扩展预留：MCP（条件项）

只有当目标 Agent **跑不了 shell**（如 Claude Desktop、ChatGPT 桌面）时才需要 MCP；能跑
shell 的平台用 CLI 即可。需要时：

1. 新建 `internal/server/mcp.go`，实现 MCP JSON-RPC 协议
2. 把 `orchestrator` / `resolver` 包成最小工具集（`add_source`、`refresh`、`query`、`report`）
3. `main.go` 增加 `sift mcp` 命令

Provider 接口、数据模型、缓存层完全不需要改动——这就是**接口设计先行**的好处。
取舍详见 `agent-integration.md`。

---

## 9. 各平台调用方式（CLI 足够）

| 平台 | 调用方式 |
|------|----------|
| **定时任务 (cron)** | `sift -q discover && sift -q report --date today -o latest.md` |
| **GitHub Actions** | 同上，在工作流文件中执行 |
| **WorkBuddy Automation** | 执行 Shell 命令，读取 stdout（JSON）或报告文件 |
| **Agent 加源** | 用户说"关注 X" → Agent 调 `sift add <input> --write` |
| **Claude Code / CLI tool use** | 配置允许的命令列表，直接调用 |
| **手动使用** | 终端直接执行 |

控制面（Agent 操作 sift）与数据面（Agent 消费结果）的完整策略见 `agent-integration.md`。
只有跑不了 shell 的平台（如 Claude Desktop）才需要 §8 的 MCP。
