# AGENTS.md

本仓库已经切换到 Go 后端实现，协作时不要再引入旧 Python 运行链路。

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
