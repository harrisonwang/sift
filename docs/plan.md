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
│    discover · query · report · config    │
│            (cobra 命令树)                 │
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
│   └── sift/
│       └── main.go                  # 唯一入口，注册 cobra 命令
├── internal/
│   ├── model/
│   │   └── item.go                  # Item 结构体
│   ├── config/
│   │   └── config.go                # YAML 配置解析
│   ├── provider/
│   │   ├── provider.go              # Provider 接口定义
│   │   ├── registry.go              # 工厂方法，根据配置激活
│   │   ├── twitter/
│   │   │   └── twitter.go
│   │   ├── hackernews/
│   │   │   └── hackernews.go
│   │   ├── reddit/
│   │   │   └── reddit.go
│   │   └── rssblog/
│   │       └── rssblog.go           # 通用 RSS 博客 Provider
│   ├── cache/
│   │   ├── cache.go                 # Cache 接口
│   │   └── sqlite.go                # SQLite 实现
│   ├── reporter/
│   │   ├── reporter.go              # Reporter 接口
│   │   ├── markdown.go
│   │   └── json.go
│   └── orchestrator/
│       └── orchestrator.go          # 核心调度：遍历 Provider → 去重 → 生成结果
├── config.example.yaml
├── go.mod
├── go.sum
├── .goreleaser.yaml
├── .github/
│   └── workflows/
│       └── release.yml
└── README.md
```

MCP/SKILL 若未来需要，可以另起 `internal/server/` 作为独立包，不影响核心。

---

## 5. CLI 命令设计

```bash
# 核心命令
sift discover --config config.yaml
sift query --date 2026-06-03 --format json
sift report --date 2026-06-03 --output report.md

# 辅助命令
sift config validate          # 验证配置文件
sift config show              # 打印当前激活的 Provider 和配置
sift source list              # 列出所有可用的 Provider
sift source info twitter      # 查看某个 Provider 的详细信息
```

- `discover`：遍历所有启用的 Provider，抓取新内容，写入缓存
- `query`：从缓存中按条件查询，输出 JSON 到 stdout
- `report`：生成 Markdown/JSON 报告文件
- `config`：配置相关的管理命令
- `source`：查看 Provider 信息

---

## 6. 配置设计

```yaml
cache:
  db_path: ./cache/sift.db

output:
  report_dir: ./reports

providers:
  - name: twitter
    enabled: true
    config:
      accounts: [msdev, openai]
      nitter_instances:
        - https://nitter.net
        - https://nitter.privacydev.net

  - name: hackernews
    enabled: true
    config:
      feeds: [top, new, show]
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
        - url: https://openai.com/blog/rss.xml
          source: openai_blog
        - url: https://www.anthropic.com/blog/feed.xml
          source: anthropic_blog
        - url: https://paperswithcode.com/feed/latest
          source: paperswithcode
        - url: https://www.producthunt.com/feed
          source: producthunt
```

---

## 7. 开发路线图

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

---

## 8. 未来扩展预留

当需要支持 MCP/SKILL 时，只需要：

1. 新建 `internal/server/mcp.go`，实现 MCP JSON-RPC 协议
2. 包装 `orchestrator` 的方法为 MCP tools（`discover`, `query`, `report`）
3. `main.go` 增加 `sift serve --mcp` 命令

Provider 接口、数据模型、缓存层完全不需要改动。这就是**接口设计先行**的好处。

---

## 9. 各平台调用方式（CLI 足够）

| 平台 | 调用方式 |
|------|----------|
| **定时任务 (cron)** | `sift discover && sift report --date today` |
| **GitHub Actions** | 同上，在工作流文件中执行 |
| **WorkBuddy Automation** | 执行 Shell 命令，读取 stdout 或报告文件 |
| **ChatGPT App (Actions)** | 调用部署好的环境中的 CLI（需一个可执行环境） |
| **Claude (CLI tool use)** | 配置允许的命令列表，直接调用 |
| **手动使用** | 终端直接执行 |

未来有了 `sift serve --mcp`，支持 MCP 的平台（如 Claude Desktop）可以免配置直接通过协议发现和调用。
