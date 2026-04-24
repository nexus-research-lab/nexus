# AGENTS.md

本仓库已经切换到 Go 后端实现，协作时不要再引入旧 Python 运行链路。

## Build & Validation Commands
- `make dev`：同时启动 Go 后端（8010）和前端（3000）
- `make check`：运行 `go test ./...`、前端 lint、前端 typecheck
- `make check-backend`：Go 后端校验，等价于 `make check-go`
- `make db-init`：执行 Goose 数据库迁移
- `make install`：执行 `go mod tidy` 并安装前端依赖
- `go run ./cmd/nexusctl ...`：主智能体操作系统 CLI

## Critical Conventions
- Go 代码遵循 Google 风格，复杂逻辑注释使用中文。
- 后端入口在 `cmd/`，业务实现放在 `internal/`，协议真相源在 `internal/protocol/`。
- 所有 API 路由统一挂在 `/agent/v1/...` 下，WebSocket 入口固定为 `/agent/v1/chat/ws`。
- 数据库迁移统一走 Goose，目录为 `db/migrations/sqlite` 和 `db/migrations/postgres`。
- Workspace 与 session JSONL 文件继续作为文件侧真相源保留。
- Go 文件命名采用“职责前缀 + 领域名”形式，方便目录按职责聚合；例如 `command_room.go`、`model_agent.go`、`service_heartbeat.go`、`repository_room.go`、`builder_prompt.go`。包内主入口文件可保留 `service.go`、`model.go`、`repository.go` 这类无歧义名称。
- 前端源码文件使用 kebab-case 的“职责前缀 + 领域名”形式；例如 `page-room.tsx`、`dialog-agent-options.tsx`、`panel-chat-dm.tsx`、`api-room.ts`、`store-workspace-live.ts`、`context-auth.ts`。React hook 文件保留 `use-*` 前缀，`index.ts`、`main.tsx`、`App.tsx`、`env.d.ts` 这类入口或 barrel 文件不强制改名。

## Architecture Flow
- 服务入口：`cmd/nexus-server`
- 迁移入口：`cmd/nexus-migrate`
- 主 CLI：`cmd/nexusctl`
- HTTP / WebSocket 网关：`internal/gateway`
- Claude Code runtime：`internal/runtime` + 独立 Go SDK
- 会话与协议：`internal/chat`、`internal/room`、`internal/session`、`internal/protocol`
- Workspace / Skills / Connectors / Automation：`internal/workspace`、`internal/skills`、`internal/connectors`、`internal/automation`

## Commit Style
提交信息保持 emoji 前缀 + 中文摘要，例如 `:sparkles: 切换 Go 默认运行链路`。用户可见变更同步更新 `CHANGELOG.md`。
