# AGENTS.md

`nexus` — 用户运行的多 agent 桌面/网页应用；Go 后端 + React web。
技术栈：Go + net/http + WebSocket + SQLite/goose + React19 + Vite + Zustand。

本文件只保存项目宪法：构建门禁、文档分层、依赖方向与工程原则。产品合同一律写在 `docs/specs/`，见文末索引；不得在本文件追加产品行为细节。

## Build & Validation Commands
- `make dev`：同时启动同级 Nexus Control（8020）、Go 后端（8010）和前端（3000）
- `make check-architecture`：检查生产导入方向与 textutil 私有副本；集成测试可继续通过 app 装配。
- `make check-go`：默认 Go 门禁，只检查相对上游及当前工作树中发生变化的 Go 包
- `make check-go-fresh`：对上述变化包禁用测试结果缓存
- `make check-go-full`：显式运行 Go 全量 vet 与无缓存测试，仅用于发布、跨包基础设施变更或用户明确要求
- `make check`：运行增量 Go 门禁、前端 lint、前端时间线行为测试、前端 typecheck
- `make check-backend`：Go 后端增量校验，等价于 `make check-go`
- `make check-normalization`：拒绝对已被证明裁剪过的值再调用 `strings.TrimSpace`（linux/darwin/windows 取交集，约 1 分钟），已并入 `make check-go-full`
- `make install`：执行 `go mod tidy` 并安装前端依赖
- `NEXUS_SANDBOX_TEST_BINARY=/absolute/nxs make check-desktop-sandbox`：显式桌面沙箱基线；macOS/Windows 原生验收入口与必测项见 `docs/testing/desktop-sandbox-acceptance.md` 与 `docs/specs/desktop-sandbox-spec.md`。

日常修改先跑目标包测试或 `make check-go`。Agent 不得默认执行 `go test ./...` 或 `make check-go-full`；只有发布、共享协议/基础设施变更、增量范围无法可靠判断，或用户明确要求全量时才执行。

## Commit Style
Use English commit messages with an emoji prefix, for example `:sparkles: Switch to the Go default runtime path`. Keep user-visible changes reflected in `CHANGELOG.md`.

## L1 — 文档地图

代码是机器相，注释是语义相，两相必须同构：任一相变化必须在另一相显现，否则视为未完成。
本仓采用三层分形文档：**L1**（本文件，项目宪法）→ **L2**（各 Go 包 `doc.go` 的 `L2` 头，成员清单 + 暴露接口）→ **L3**（业务文件顶部 `INPUT/OUTPUT/POS` 契约）。跨包产品语义只在 `docs/specs/` 保留一份当前规范；`internal/protocol` 类型、`nexus.command` MCP schema 与 parser 是线格式真相，Skill 只说明模型决策，API Reference 只说明 transport。未来方案、交付计划和未实现字段必须明确标为 non-normative，不能混入当前规范或由多份文档重复定义。

```
<directory>
cmd/        - 可执行入口：nexus-server（服务 + 自动迁移）、nexusctl 资源 CLI、nexuscfg 配置 CLI、Linux runtime launcher
web/        - React 前端（features / store / shared / lib，见 web/AGENTS.md）
desktop/    - macOS AppKit/WKWebView、Windows WPF/WebView2 宿主与 Chromium browser-extension
skills/     - 随产品发布的平台内置 Skill（每个目录自含 SKILL.md、元数据、脚本与参考资料）
internal/   - 后端核心（各子包 L2 见其 doc.go）:
  protocol/   - 跨 HTTP/WS/前端/运行时的协议真相源与 TS codegen 输入
  runtime/    - nxs/Claude Code 共用宿主主链：bridge client、manager 生命周期、workspace isolation Hook、桌面沙箱资源
  service/    - 业务服务；service/room 只持久化 Room，实时编排在 service/room/realtime
  chat/       - 对话领域（dm / room）
  handler/    - HTTP / WebSocket 处理器；team 是浏览器到可选多人服务的认证 gateway
  relay/      - Relay 独立跨仓合同；service/relay 是客户端，service/team 负责远端结果与本地投影同步
  message/    - runtime/SDK 消息 → Nexus 事件与 assistant 快照的投影
  echo/ automation/ - 主动跟进与定时任务领域模型
  mcp/        - 宿主自有工具统一挂在单一 `nexus` MCP server；`nexus.command` 承载 Goal/Execution/Automation/Subagent
  cli/        - nexusctl / nexuscfg 本地命令行装配；模型侧命令不经过 CLI
  app/        - HTTP 与 CLI 共用的显式服务装配、资源所有权与进程生命周期
  config/ storage/ infra/ migration/ version/ - 装配、持久化、基础设施与版本化迁移；infra/textutil 是只依赖标准库的字符串原语叶子包
docs/       - README.md 是索引；guides/ 面向用户与作者，operations/ 面向运维，testing/ 保存回归清单与证据，specs/ 保存当前维护者合同，explorations/ 保存 non-normative 在研专题
</directory>
```

[PROTOCOL]: 变更时更新此头部，然后检查各 Go 包入口 `doc.go`（L2）

## 状态根契约

- `.nexus` 是统一 `NEXUS_STATE_ROOT`；宿主数据位于 `.nexus/app`，独立 Control 数据位于 `.nexus/control`，公钥镜像位于 `.nexus/control-public`，用户数据位于 `.nexus/users/<owner>/`。
- 启动只把当前 canonical 布局作为运行时读写路径；历史数据只能经 `internal/migration` 中版本化、可重试、不提供旧路径回读的迁移进入 canonical 布局，迁移必须允许跨版本直接升级。
- 宿主代 runtime 操作 workspace、transcript、artifact、用户 Skill 或 Room 状态时必须使用 `internal/infra/confinedfs`。
- 账号、组织、订阅与在线 Team 的权威位于 `nexus-control`/Relay，Nexus 只保存本地投影；细节见 `docs/specs/online-team-spec.md`。目录布局与隔离细节见 `docs/specs/workspace-isolation-spec.md`。

## 后端依赖方向

```text
cmd -> app -> handler -> service -> domain/storage
                 \-> protocol <- runtime/message
```

- `app` 只负责装配、路由和进程生命周期，不承载业务规则。
- `handler` 在消费侧定义小接口，只依赖当前端点需要的操作；实现返回具体类型。
- `service` 负责业务阶段和事务边界，不依赖 `handler` 或 `app`；`service/room/realtime` 只能依赖 `service/room`，不能反向。
- `storage` 负责持久化与数据库方言，共享 SQL 分叉统一进入 `SQLDialect`，领域查询留在各自 repository。
- `runtime` 只描述 bridge 会话与执行生命周期；SDK 系统消息到产品事件的投影统一属于 `message`。

## 内部依赖门禁

当前边界与验收见 `docs/specs/internal-boundaries.md`，由 `scripts/check-architecture` 检查生产导入并拒绝按形状识别的 textutil 私有副本，并接入增量 Go 检查与全量 vet 入口。

- protocol 与 relay 合同不依赖其他 internal 包；runtime 根包只消费 protocol，并通过 `internal/infra/confinedfs` 使用固定目录句柄完成宿主沙箱资源的创建、标记和回收；除该明确文件边界与只依赖标准库的 `internal/infra/textutil` 叶子包外不得引入其他 infra/service 依赖；textutil 自身不得依赖任何 internal 包。
- service 不依赖 app/handler；storage、infra、message 不依赖 app/handler/service，message 也不依赖 storage。
- orchestration 核心不依赖 MCP，协议转换进入 runtimehook；app 共享装配不反向依赖 app/server。
- Session 跨表清理使用调用方持有的同一事务，SQL 归本领域仓储，不能由各服务分别提交。

## 工程原则

- 长流程按业务阶段拆成私有函数，阶段之间传递有语义的结构体；一个产品语义只保留一个投影入口。Go 文件不设机械行数上限，按业务内聚、依赖边界和阅读路径决定拆合；同一业务散落时优先合并，不以透传参数包或多层薄包装掩盖复杂度。
- DM 是 Room 的一种：两者共用的运行阶段只实现一次，Room 只注入多成员 slot 与公私域策略。
- 共享字符串原语只用 `internal/infra/textutil`，不得私有复制。
- 字符串只在入口裁剪一次：handler/WebSocket 解析、MCP parser、存储扫描与 `authctx` 身份读取负责清洗，下游一律信任；不得在 service 内部对已清洗的值重复 `strings.TrimSpace`。
- 测试便利入口优先留在 `_test.go`；只有跨包集成测试需要共享装配时，才在生产包保留窄入口。新增测试应覆盖尚未覆盖的分支，不重复已覆盖路径。
- 数据影响只能由事务、revision、durable ACK 或领域回执证明；断线/超时进入 unknown 并先对账，禁止自动重放副作用（见 `docs/specs/failure-recovery-spec.md`）。

## 产品合同索引

| 主题 | 唯一规范 |
| --- | --- |
| 账号、组织、订阅、在线 Team、Node | `docs/specs/online-team-spec.md` |
| 状态根布局、Linux 隔离、宿主文件边界 | `docs/specs/workspace-isolation-spec.md` |
| 桌面沙箱、macOS 进程恢复、记忆写入 | `docs/specs/desktop-sandbox-spec.md` |
| Goal / Execution / WorkGraph / Subagent | `docs/specs/execution-orchestration-spec.md` · `docs/specs/execution-graph-spec.md` |
| Room 持久化、侧栏活动、Room-backed Session | `docs/specs/room-spec.md` · `docs/specs/room-collaboration-spec.md` |
| 消息链路、上下文块、会话 UI 呈现 | `docs/specs/message-processing-spec.md` |
| 失败协议、Automation 持久阶段 | `docs/specs/failure-recovery-spec.md` |
| 定时任务绑定与 IM 投递权限 | `docs/specs/automation-permission-pipeline-spec.md` |
| 运行时人工交互与 MCP 工具面 | `docs/specs/permission-runtime-spec.md` |
| 对话式配置、revision 密钥 | `docs/specs/conversational-configuration-control-spec.md` |
| Slash 与计划模式 | `docs/specs/slash-command-spec.md` |
| 能力页 | `docs/specs/capability-page-design-spec.md` |
| 自动审核 | `docs/auto-review.md` |
| 包边界、MCP 能力域、测试组织 | `docs/specs/internal-boundaries.md` |
