# AGENTS.md

## Build & Validation Commands
- `make dev`：同时启动 Go 后端（8010）和前端（3000）
- `make check`：运行 `go test ./...`、前端 lint、前端 typecheck
- `make check-backend`：Go 后端校验，等价于 `make check-go`
- `make install`：执行 `go mod tidy` 并安装前端依赖

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
  protocol/   - 跨 HTTP/WS/前端/运行时的协议真相源（会话/房间/Goal 模型与 Room creator/lead 身份、事件、枚举、TS codegen 输入）
  runtime/    - nxs/Claude Code 共用宿主主链（bridge client、manager 会话/round 生命周期）
  service/    - 业务服务（agent / dm / room / session / workspace / skills / connectors / automation / llm ...）
  chat/       - 对话领域（dm / room）
  handler/    - HTTP / WebSocket 处理器
  message/    - runtime/SDK 消息 → Nexus 事件与 assistant 快照的映射投影
  automation/ - 定时任务 / heartbeat 调度域
  service/memorymaintenance/ - Nexus 唤醒 nxs 后台记忆维护的宿主协调器
  cli/        - nexusctl 命令装配（按领域文件组织）
  app/        - HTTP 服务装配与生命周期
  mcp/ connectors/ workspace/ - 能力域
  config/ storage/ infra/ migration/ version/ - 装配、一次性迁移与基础
docs/       - 跨切面设计文档
</directory>
```

[PROTOCOL]: 变更时更新此头部，然后检查各 Go 包入口 `doc.go`（L2）

## 后端依赖方向

```text
cmd -> app -> handler -> service -> domain/storage
                 \-> protocol <- runtime/message
```

- `app` 只负责装配、路由和进程生命周期，不承载业务规则。
- `handler` 在消费侧定义小接口，只依赖当前端点需要的操作；实现返回具体类型。
- `service` 负责业务阶段和事务边界，不依赖 `handler` 或 `app`。
- `storage` 负责持久化与数据库方言，不保留没有行为的方言门面；共享 SQL 分叉统一进入 `SQLDialect`，领域查询留在各自 repository。
- `runtime` 只描述 bridge 会话与执行生命周期；SDK 系统消息到产品事件的投影统一属于 `message`。
- 测试便利入口优先留在 `_test.go`；只有跨包集成测试需要共享装配时，才在生产包保留窄入口。

长流程按业务阶段拆成私有函数，阶段之间传递有语义的结构体；一个产品语义只保留一个投影入口。Go 文件不设机械行数上限，按业务内聚、依赖边界和阅读路径决定拆合；同一业务散落时优先合并，不以透传参数包或多层薄包装掩盖复杂度。
