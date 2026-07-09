# AGENTS.md

## Build & Validation Commands
- `make dev`：同时启动 Go 后端（8010）和前端（3000）
- `make check`：运行 `go test ./...`、前端 lint、前端 typecheck
- `make check-backend`：Go 后端校验，等价于 `make check-go`
- `make install`：执行 `go mod tidy` 并安装前端依赖
- Agent runtime 内使用 `NEXUSCTL_COMMAND_PATH` 指向的 CLI；本仓库开发调试时才使用 `go run ./cmd/nexusctl ...`

## Critical Conventions
- Go 代码遵循 Google 风格，复杂逻辑注释使用中文。
- 后端入口在 `cmd/`，业务服务放在 `internal/service/`，协议真相源在 `internal/protocol/`。
- `internal/protocol` 只放跨 HTTP/WebSocket/前端/运行时边界共享的协议模型、枚举、事件构造和代码生成输入；服务内部输入、仓储 DTO、持久化 codec 留在对应 `internal/service/*` 或 `internal/storage/*`。
- 同目录 Go 文件按职责前缀命名，例如 `model_xxx.go`、`service_xxx.go`、`command_xxx.go`、`repository_xxx.go`、`factory_xxx.go`、`constant_xxx.go`。

## Architecture Flow
- 服务入口：`cmd/nexus-server`
- 数据库迁移：`cmd/nexus-server` 启动时自动执行
- 主 CLI：`cmd/nexusctl`
- HTTP 服务装配与生命周期：`cmd/nexus-server/app`
- HTTP / WebSocket 处理器：`internal/handler`
- Claude Code runtime：`internal/runtime` + 独立 Go SDK
- 业务服务：`internal/service/agent`、`internal/service/dm`、`internal/service/room`、`internal/service/session`
- 对话领域：`internal/chat/dm`、`internal/chat/room`
- 能力服务：`internal/service/workspace`、`internal/service/skills`、`internal/service/connectors`、`internal/service/automation`
- 协议与执行内核：`internal/protocol`、`internal/runtime`、`internal/message`、`internal/permission`

## Commit Style
Use English commit messages with an emoji prefix, for example `:sparkles: Switch to the Go default runtime path`. Keep user-visible changes reflected in `CHANGELOG.md`.

## L1 — 文档地图

代码是机器相，注释是语义相，两相必须同构：任一相变化必须在另一相显现，否则视为未完成。
本仓采用三层分形文档：**L1**（本节，项目宪法）→ **L2**（各 Go 包 `doc.go` 的 `L2` 头，成员清单 + 暴露接口）→ **L3**（业务文件顶部 `INPUT/OUTPUT/POS` 契约）。

`nexus` — 用户运行的多 agent 桌面/网页应用；Go 后端 + React web。
技术栈: Go + net/http + WebSocket + SQLite/goose + React19 + Vite + Zustand

```
<directory>
cmd/        - 可执行入口（nexus-server 服务 + 自动迁移；nexusctl 命令行）
web/        - React 前端（features / store / shared / lib，见 web/CLAUDE.md）
internal/   - 后端核心（各子包 L2 见其 doc.go）:
  protocol/   - 跨 HTTP/WS/前端/运行时的协议真相源（会话/房间/Goal 模型、事件、枚举、TS codegen 输入）
  runtime/    - Claude Code runtime 主链（executor_round 主链、manager 会话/round 生命周期）
  service/    - 业务服务（agent / dm / room / session / workspace / skills / connectors / automation / llm ...）
  chat/       - 对话领域（dm / room）
  handler/    - HTTP / WebSocket 处理器
  message/    - runtime/SDK 消息 → Nexus 事件与 assistant 快照的映射投影
  automation/ - 定时任务 / heartbeat 调度域
  cli/        - nexusctl 命令装配（command_* 各命令域）
  app/        - HTTP 服务装配与生命周期
  mcp/ connectors/ workspace/ - 能力域
  config/ storage/ infra/ version/ - 装配与基础
docs/       - 跨切面设计文档
</directory>
```

[PROTOCOL]: 变更时更新此头部，然后检查各 Go 包入口 `doc.go`（L2）
