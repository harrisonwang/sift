# sift × Agent:核心方案

> 架构见 `plan.md`;keystone 功能 `sift add` 的详设见 `sift-add.md`。

## 核心问题

用户在聊天里让 Agent **管理**自己的信息源:加源、验证、即时查 / 刷新、按需出报告。
本质是**控制面**问题:`自然语言意图 → 工具调用 → 改 sift 状态`。

## 控制面 vs 数据面

- **控制面(核心)**:Agent **操作** sift —— `add` / 验证 / `discover` / `query` /
  `report` 按需调用。需要 **action channel**(能跑 shell + 知道命令)。
- **数据面(辅助)**:Agent **消费**结果 —— 读 `latest.md` / `latest.json`。cron 一行即可。

> cron / push 只解决数据面,解决不了"让 Agent 知道、并去管理雷达"。

## 优先级(收敛)

1. **`sift add <input...> --json [--write]`** —— 控制面基石(**MVP 已实现**)。
2. **push digest**(cron:`discover → report -o latest.md`)—— 数据面辅助。
3. **awareness 注入**(Skill / system-prompt / 一次性提示)—— 让 Agent 知道何时用 sift。

> **MCP 暂不做**:只有当目标 Agent **跑不了 shell** 时才需要它;能跑 shell 的平台,CLI
> 就够。需要时再加,包同一套 orchestrator。

## 跨平台适配

CLI + JSON 是**平台中立**的:同一条命令、同一份 JSON,在任何能跑 shell 的平台
(WorkBuddy 等)上表现一致。每个平台只需各自做一点 awareness 注入。跑不了 shell 的平台
不在当前范围。

## 不可控 Agent 的边界

- 控制面**必须有 action channel**(原生工具 / shell / 平台插件);没有就改不了 sift 状态,
  push 绕不开这一层。
- 你不拥有的 Agent **没有完美解**——只能影响、不能保证。能可靠掌控的只有:① 自研 Agent
  做原生工具;② 自己插一层可控编排,把第三方退化成纯对话壳。

## 自进化(远期)

`sift add` 解析不了的源 → 后台 agent 开 issue → Coding Agent(Claude Code / Codex)
clone / 写 / `gofmt` / `build` / `test` → 开 PR。**切在 PR 处**:合并 + 发布留人或硬门
(等有了 provider 契约测试再谈自动合并)。而且多数"不支持"用 `sift add` + RSS 自动发现 +
(未来)通用 `httpjson` / `exec` provider 就能变成**配置而非代码**——先做这层,需求塌大半。

## 一句话

**主线 = `sift add`(控制面)+ cron push(数据面);awareness 注入让 Agent 知道用它。
自进化是远期,且切在 PR。**
