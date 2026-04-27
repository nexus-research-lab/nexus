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
- 后端入口在 `cmd/`，业务服务放在 `internal/service/`，协议真相源在 `internal/protocol/`。
- `internal/protocol` 只放跨 HTTP/WebSocket/前端/运行时边界共享的协议模型、枚举、事件构造和代码生成输入；服务内部输入、仓储 DTO、持久化 codec 留在对应 `internal/service/*` 或 `internal/storage/*`。
- 同目录 Go 文件按职责前缀命名，例如 `model_xxx.go`、`service_xxx.go`、`command_xxx.go`、`repository_xxx.go`、`factory_xxx.go`、`constant_xxx.go`。

## Architecture Flow
- 服务入口：`cmd/nexus-server`
- 迁移入口：`cmd/nexus-migrate`
- 主 CLI：`cmd/nexusctl`
- HTTP / WebSocket 网关：`internal/gateway`
- Claude Code runtime：`internal/runtime` + 独立 Go SDK
- 业务服务：`internal/service/agent`、`internal/service/chat`、`internal/service/room`、`internal/service/session`
- 能力服务：`internal/service/workspace`、`internal/service/skills`、`internal/service/connectors`、`internal/service/automation`
- 协议与执行内核：`internal/protocol`、`internal/runtime`、`internal/message`、`internal/permission`

## Commit Style
提交信息保持 emoji 前缀 + 中文摘要，例如 `:sparkles: 切换 Go 默认运行链路`。用户可见变更同步更新 `CHANGELOG.md`。
