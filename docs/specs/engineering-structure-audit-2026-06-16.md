# Nexus 工程结构审计与迁移记录

日期：2026-06-16

## 审计口径

本记录关注整个项目的工程化结构，而不是单点功能正确性。重点判断：

- 目录和包边界是否表达清楚。
- 入口、协议、业务服务、存储、运行时是否分层稳定。
- 单文件是否承担过多职责。
- 是否存在重复实现、双轨架构、跨层依赖或迁移残留。
- app / web / Docker 是否存在明显性能优化空间，尤其是启动、构建、运行时 IO、缓存和网络请求路径。
- 如果需要迁移，迁移是否可以分阶段、可验证、低风险推进。

本轮只在本地工作树记录和整理，不推送远端。

## 当前结论摘要

整体方向是健康的：Go 后端已经形成 `cmd`、`internal/handler`、`internal/service`、`internal/storage`、`internal/protocol`、`internal/runtime` 的主干分层。但项目正在快速扩展，部分域已经出现“业务服务过厚、协议模型过大、存储实现重复、运行时核心过宽”的结构风险。

第一轮已处理的结构债：

- `internal/service/connectors/service.go` 已拆分为目录、连接、OAuth、Device Flow、credential payload 等同包文件。
- app server 的 Go 静态托管已补齐 `/assets/*` immutable cache 和 HTML fallback no-cache，避免不经 nginx 时重复拉取哈希资源。
- `internal/service/auth/service.go` 已拆成 core、session、user、principal、desktop、cookie、token、validate 等同包文件。
- `internal/service/imagegen/service.go` 已拆成 core、config、provider call/options、HTTP retry、image extract、input normalize、output write 等同包文件。
- `internal/service/provider/service_test.go` 已按 catalog/runtime、visibility/admin、模型发现、模型变更、HTTP payload/redaction 和共享夹具拆成同包测试文件，避免 provider 回归测试继续混成单文件。
- `internal/storage/provider/repository.go` 已拆成 provider CRUD/query、model 仓储、runtime usage、scan codec、dialect helper 等同包文件。
- `internal/runtime/executor_round.go` 已拆成主执行循环、model、stream diagnostics、stream error、terminal result、util 等同包文件。
- `internal/workspace/memory/engine.go` 已拆成 query、mutation、capture、item、scope/score、render、message extraction、util 等同包文件。
- `internal/service/room/service.go` 已拆成 query、room CRUD、member、conversation、cleanup、agent resolution、host settings 等同包文件。
- `internal/service/room/goal_runtime_test.go` 已按 usage accounting、slot runtime context、continuation/collaboration 和共享夹具拆成同包测试文件。
- `internal/service/room/service_test.go` 已按 lifecycle/runtime、skills/host/direct room、artifact cleanup 和共享夹具拆成同包测试文件。
- `internal/storage/sqlite/repository_room.go` 与 `internal/storage/postgres/repository_room.go` 已分别保留 dialect SQL 主体并拆出 delete、load 分片；过薄 util helper 已合回主文件，主文件约 751 / 752 行。
- `internal/protocol/model_automation.go` 已拆成常量/错误、schedule、target/source/delivery、job/run/status、daily report、create/update input、heartbeat/system event 等同包文件。
- `internal/service/session/service.go` 已拆成 query、mutation、history、workspace、runtime、util、model 等同包文件，入口文件只保留 Service 核心和构造。
- `internal/service/session/service_test.go` 已按 lifecycle/listing/title、history/meta 和共享夹具拆成同包测试文件。
- `internal/service/dm/service_test.go` 已按 round runner/external reply、goal runtime、chat、runtime session、interrupt/input queue 和共享夹具拆成同包测试文件，避免 4639 行单测试文件继续承载所有回归。
- `internal/service/dm/service_test_helpers_test.go` 已按 runtime fake、goal fake、event/assertion helper 和环境/DB/transcript fixture 继续收敛，避免共享 helper 变成新的 996 行垃圾桶。
- `internal/service/room/service_realtime_test.go` 已按 chat delivery、chat runtime、mention queue、interrupt、SDK session 和共享夹具拆成同包测试文件，避免 3454 行实时回归测试继续混放所有场景。
- `internal/service/room/input_queue.go` 已拆成入口控制、dispatch、context、store、broadcast、util 等同包文件。
- `internal/service/room/execution.go` 已拆出 runtime session、diagnostics、tool policy 等辅助职责，入口文件只保留 round / slot 主执行流程。
- `internal/service/room/chat.go` 已拆出 target/title/agent directory/persistence；`round_state.go` 已拆出 registry、mapper、slot state/input/interrupt helper。
- `internal/handler/websocket/handler.go` 已拆成连接 keepalive、dispatch、app event subscription、room subscription、session binding、control message、error event、value parser 等同包文件。
- IM 的 `feishu` / `wecom_bot` 大适配器已拆成 channel、api、安全校验、ingress、frame/socket/model 等职责文件。
- IM 的 `router.go` / `ingress.go` 已拆成注册、生命周期、route memory、delivery、accept、normalize、session、permission、delivery target 等同包文件。
- IM 的 pairing / login 服务已拆成 CRUD、ingress resolve、store、view/stats、login flow、个人微信执行流程、login store、session state 等同包文件。
- IM 的 DingTalk adapter 已拆成 core、delivery/token、stream callback、HTTP callback decode、util 等同包文件。
- IM 的 Telegram adapter 已拆成 channel core、delivery/typing、polling、ingress、model、util 等同包文件。
- IM 的 ingress envelope 测试已按 Discord、Telegram、DingTalk 拆成平台文件。
- IM 的 ingress service 测试已按基础 accept/session、个人微信隔离、权限策略、reqID 幂等和 helpers 拆分。
- IM 的 control service 测试已按 catalog、config、login、pairing、Feishu ingress 和 helpers 拆分，避免 1395 行单测试文件继续膨胀。
- automation 的 task / heartbeat 文件已按 query、crud、run、support 和 status、dispatch、runtime 拆分。
- automation 的 execution / observability 文件已按执行启动、session dispatch、观察收尾、overlap skip、日报、健康诊断和 helper 拆成同包职责文件。
- automation execution 测试已按 run ledger/overlap、room/main/script target 和权限/owner scope 拆分，避免运行账本、目标路由和访问控制混在一个测试文件。
- automation delivery 测试已按 observation、success/inbox、retry、dead-letter 和 helpers 拆分，避免 1171 行单测试文件继续膨胀。
- automation recovery 测试已按 runtime claim/state、recovery runtime 和 cleanup 拆分，避免运行态领取、恢复和删除清理继续混在一个 1041 行测试文件。
- automation MCP management 测试已按 disable、enable/delete、list/search 工具域拆分，避免管理写操作和查询/历史搜索混在一个 1015 行测试文件。
- automation MCP observability 测试已按 daily report、events/status、run 生命周期拆成 3 个职责文件，run 相关场景合并到一个文件，避免为了行数切出 50/116 行小碎片。
- `internal/storage/auth/repository.go` 已拆成 repository core、state、user、password credential、session、value helper 等同包职责文件。
- `internal/service/skills/marketplace_search.go` 已拆成搜索聚合、source-specific HTTP 查询、外部索引 row 映射、过滤去重等同包职责文件。
- `internal/cli/command_skill.go` 已拆成 skill 命令树、external import 命令、agent 推断 helper 等同包职责文件。
- `internal/cli/command_memory.go` 已拆成 memory 命令树、查询/统计命令、写入命令、scope/field helper 等同包职责文件。
- `internal/infra/logx/handler_pretty.go` 已拆成 handler facade、render、field extract、color、value/format、model 等同包职责文件。
- `internal/runtime/debug_message.go` 已拆成公开 facade、日志字段构造、摘要生成、value/raw helper 等同包职责文件。
- `internal/message/processor_test.go` 已按 assistant/stream 状态、tool result/artifact/alias 和 error/permission 语义拆成同包测试文件。
- `internal/connectors/feishudocx/render_markdown.go` 已拆成 document target、renderer core、block accessor、inline style、table/media、markdown util 等同包职责文件。
- connectors service 测试已按 catalog/state、OAuth config、device flow、connection credentials、OAuth callback 和 helpers 拆分，避免 1290 行单测试文件继续膨胀。
- goal 的 progress stale-version retry 重复逻辑已抽成一个本地 helper。
- `internal/runtime/manager.go` 已按 client adapter、session、round、goal accounting、interrupt、streaming input、MCP restart check 拆成同包文件。
- `internal/storage/workspace/store_input_queue.go` 已拆成入口、JSONL file、replay、codec、order 等同包职责文件。
- `internal/storage/workspace/history_normalize.go` 已拆成 normalize facade、pagination、result summary、external delivery、unfinished round、order 等同包职责文件。
- `internal/service/workspace/manager_live.go` 已拆成订阅生命周期、fsnotify watcher、API/agent write flush、listener/path util、diff stats 等同包职责文件。
- `internal/service/workspace/service.go` 已拆成 Service 核心、model、agent workspace、file、mutation、upload、path helper 等同包职责文件。
- `internal/service/workspace/initializer_workspace.go` 已拆成初始化编排、模板、skill 同步、nexusctl shim、util 等同包职责文件。
- `internal/storage/workspace/store_session_file.go` 已拆成 session meta CRUD、JSONL 读写、history compact、value coercion 等同包职责文件。
- `internal/service/conversation/titlegen/service.go` 已拆成 service 入口、contract、model、request、preview、generation、apply、title rules 等同包职责文件。
- `internal/chat/room/visible_context.go` 已拆成动态上下文编排、稳定 prompt、公区 batch、directed message、格式化、history 文本、value helper 等同包职责文件。
- `web/src/features/capability/channels/channels-directory.tsx` 已拆出 channel model、channel icon/style、接入指南、扫码登录、账号列表和频道卡片组件，入口从 1164 行降到 232 行。
- `web/src/features/home/home-sidebar-panel.tsx` 已拆出目录加载/WebSocket 订阅、conversation/unread 建模和 chat/contact row UI，入口从 1045 行降到 418 行。
- `web/src/features/conversation/shared/message/item/message-item-model.ts` 已拆出 stats/result、permission 匹配、assistant ordering 和 activity 摘要 helper，入口从 1138 行降到 738 行。

仍需继续审计的高优先级区域：

- `internal/service/room`、`internal/service/session`、`internal/service/dm`：session service 与 room input queue / execution / chat / round state 已完成第一轮拆分；后续重点转为 workspace 和存储大文件。
- `internal/runtime`：`manager.go` 与 `executor_round.go` 已拆分；后续重点转为 runtime 与 room/session 的边界和回归覆盖。
- `internal/storage/sqlite` 与 `internal/storage/postgres`：Room repository 已完成第一轮减重，重复 scanner / aggregate 选择 helper 已收敛到 `internal/storage/roomrepo`，dialect SQL 主体继续分开。
- `internal/protocol`：automation 模型已完成第一轮同包拆分；后续只需在新增协议时继续保持按 domain 分文件。
- `internal/service/workspace` 与 `internal/storage/workspace`：workspace service、agent history、input queue、history normalize 和 live manager 已拆分；initializer、session file 仍需审计边界。
- 性能维度：Web chunk 已有 route lazy 和重依赖 lazy import；后续重点是 KaTeX 字体体积、mermaid/cytoscape chunk、Docker 双 Web build 和 runtime 镜像 profile。

本轮新增判断：

- `runtime.Manager`、`runtime.ExecuteRound`、runtime debug message、message processor tests、workspace service、workspace initializer、workspace memory engine、workspace input queue、workspace history normalize、workspace live manager、workspace session file store、conversation titlegen、room visible context、room service、room service tests、room goal runtime tests、DM service tests、DM test helpers、room realtime tests、provider service tests、session service tests、room input queue / execution / chat / round state、websocket handler、room repository、room repository scanner、protocol automation、session service、automation execution / observability、automation recovery tests、automation MCP management tests、automation MCP observability tests、auth storage repository、marketplace search、command skill、command memory、logx pretty handler、Telegram adapter、IM ingress service tests、IM ingress envelope tests 和 Feishu Docx markdown render 已完成同包职责拆分，当前后端下一优先级是 workspace storage / service 剩余偏大文件复核。
- `storage` 的 Room 双实现已完成第一轮同包拆分，公共 scanner / aggregate 选择 helper 已提到 `internal/storage/roomrepo`，后续继续避免模糊 dialect SQL 差异。
- `workspace` 的 service facade、initializer、agent transcript/history 读取、input queue store、session file store、history normalize 和 live manager 已完成第一轮拆分，后续只需控制新分片继续增长。
- 前端结构目录清楚，但部分 feature 组件和 hook 过厚，需要按 container / state / view / model 拆分。

## 文件规模信号

本轮扫描显示，生产代码中仍需重点关注：

- `web/src/shared/i18n/messages.ts`：1580 行。
- `web/src/hooks/agent/use-agent-conversation.ts`：1243 行。
- `desktop/macos/Sources/NexusDesktop/Update/DesktopUpdateChecker.swift`：1112 行。
- `web/src/features/settings/provider-settings-panel.tsx`：682 行。
- `web/src/features/conversation/shared/composer-panel.tsx`：673 行。
- `web/src/shared/ui/select-menu.tsx`：574 行。
- `web/src/shared/ui/sidebar/sidebar-wide-panel.tsx`：521 行。
- `web/src/features/conversation/shared/editor/editor-panel.tsx`：517 行。
- `internal/storage/sqlite/repository_room.go`：751 行。
- `internal/storage/postgres/repository_room.go`：752 行。

行数不是单独的坏味道，但这些文件需要逐个判断是否存在职责混合。

## 迁移原则

- 优先同包拆文件，其次抽未导出小对象，最后才迁移目录或改 package。
- 不先做跨包大搬迁，除非能证明当前 package 边界已经阻碍测试、复用或理解。
- 对 handler、service、storage、protocol、runtime 的公开接口保持兼容，迁移通过适配层分阶段完成。
- 每个迁移 slice 必须有对应专项测试，再跑 `go test ./...`。
- 不用行数驱动重构；以职责边界、依赖方向和重复实现为主。

## 后端分层审计

### Provider 服务

当前状态：

- `internal/service/provider/service.go` 同时承担 Provider 列表、选项、可用性、CRUD、运行时配置解析、图片配置解析、默认模型选择、可见性权限、删除替换和 DTO 转换。
- `internal/service/provider/service_models.go` 同时承担远端模型拉取、本地模型 CRUD、模型测试请求、endpoint 拼接、payload 构造、模型卡解析、能力 codec、敏感信息脱敏。
- `catalog_provider.go` 目前主要是 preset 数据和少量格式判断，可接受。

判断：

- 这是服务层职责混合，不是目录层级错误。
- 不需要迁出 `internal/service/provider`，但需要同包拆文件。
- `service_models.go` 里模型卡解析和 HTTP 测试 payload 是最值得先拆的纯逻辑区域，风险低、测试已有覆盖。

建议迁移：

- `service_lifecycle.go`：构造、logger、HTTP client 注入。
- `service_listing.go`：`List`、`ListOptions`、`Availability`、record conversion。
- `service_mutation.go`：Provider `Create`、`Update`、`Delete`、visibility / management 权限。
- `service_runtime_config.go`：runtime / LLM / image config 解析和默认选择。
- `service_models_discovery.go`：`FetchModels`、远端 `/models` 请求和模型合并。
- `service_models_mutation.go`：`UpdateModel`、`SetDefaultModel`、`TestProvider`、`TestModel`。
- `model_card.go`：模型卡解析、capability 推断、category 归一化。
- `provider_http.go`：endpoint、headers、minimal payload、HTTP 错误脱敏。

### Room / DM / Session

当前状态：

- `internal/service/room` 已有较多职责文件，整体比 provider 更工程化。
- `service.go` 仍承担 room/conversation/member CRUD、级联清理、agent 解析、host 配置归一化等入口级逻辑。
- `input_queue.go` 已拆成入口、dispatch、context、store、broadcast、util；`execution.go` 已拆出 runtime session、diagnostics、tool policy；`chat.go` 已拆出 target/title/directory/persistence；`round_state.go` 已拆出 round registry/mapper、slot state/input/interrupt helper。
- `internal/service/session/service.go` 已拆成 query、mutation、history、workspace、runtime、util、model 等同包文件，入口文件只保留依赖和构造。
- `internal/service/dm` 拆分较好，入口 `service.go` 主要是依赖和 setter，结构风险低于 room/session。

判断：

- room 不建议迁移目录；它已经是一个聚合域。
- session 是兼容层，技术债来自“双会话来源”：workspace 文件 session 与 SQL room session；第一轮文件边界已清楚，后续应控制新增规则继续落到对应分片。
- 如果后续 room 成为主会话模型，session service 应逐步收缩为查询 facade，而不是重新承载业务规则。

建议迁移：

- `room/service.go` 拆成 `service_room_crud.go`、`service_conversation_crud.go`、`service_member.go`、`service_cleanup.go`、`service_agent_resolution.go`。
- 已落地 `session/service_query.go`、`service_mutation.go`、`service_history.go`、`service_workspace.go`、`service_runtime.go`、`service_util.go`、`service_model.go`。
- 长期方向：把 room SQL session 作为主会话视图，workspace 文件 session 只作为 legacy / transcript source。

### Runtime

当前状态：

- `internal/runtime/manager.go` 已收缩为 manager state、构造和内部状态初始化。
- SDK client adapter、client lifecycle、MCP server set restart 判断、session get/replace/close、round tracking、goal accounting、interrupt、running round input 已拆到同包职责文件。
- `executor_round.go` 已只保留主执行循环；round 错误模型、请求/结果模型、stream 诊断、terminal 判断、client content helper 已拆分。
- `runtime/permission` 已经分成 context/request/presenter，结构相对健康。

判断：

- `runtime.Manager` 第一轮结构风险已解除，后续新增能力应继续放入对应 manager 分片，避免入口文件重新变厚。
- `executor_round.go` 的第一轮结构债已解除，后续关注 runtime 与 room/session 的行为边界。
- 不建议把 manager 拆出 package；同包文件拆分即可。

建议迁移：

- 已落地 `manager_client.go`：`Client`、`Factory`、`sdkClientAdapter`、SDK session pump、transport close 判断。
- 已落地 `manager_session.go`：`GetOrCreate`、runtime client replace、session close。
- 已落地 `manager_round.go`：`StartRound`、`MarkRoundFinished`、running round 查询。
- 已落地 `manager_goal_accounting.go`：flush / clear / activate goal accounting。
- 已落地 `manager_interrupt.go`：interrupt、force cancel、interrupt reason。
- 已落地 `manager_streaming_input.go`：向运行中 round 发送 content。
- 已落地 `manager_mcp.go`：managed goal MCP server set restart 判断。
- 已落地 `executor_round_model.go`、`executor_round_stream_diagnostics.go`、`executor_round_stream_error.go`、`executor_round_terminal.go`、`executor_round_util.go`。

### Storage

当前状态：

- `internal/storage/sqlite/repository_room.go` 和 `internal/storage/postgres/repository_room.go` 仍是双 dialect 实现，主要差异是占位符和少量 SQL 表达。
- 两个主文件已拆出级联删除、aggregate 装配、ID/placeholder util；重复 scan / aggregate 选择 helper 已收敛到 `internal/storage/roomrepo/scan_room.go`，主文件保留 room CRUD、conversation CRUD、member 操作及 dialect SQL 主体。
- `internal/storage/workspace/store_agent_history.go` 已完成 facade / overlay / transcript / marker / path 拆分。
- `internal/storage/workspace/history_normalize.go` 已完成 normalize facade、pagination、result summary、external delivery、unfinished round、order 拆分；`store_session_file.go` 已拆成 session meta、JSONL、history compact、value coercion，同包 storage 大文件风险已明显降低。

判断：

- Room repository 第一轮同包分片方向正确，不应强行合并成一个 SQL 字符串大模板；当前已把重复 scanner 和 aggregate 选择 helper 收敛到 `internal/storage/roomrepo/scan_room.go`，SQL 差异继续留在各 dialect 包内。
- AgentHistoryStore 第一轮结构风险已解除；后续继续控制 transcript / overlay / path 分片增长即可。

建议迁移：

- Room storage：
  - 已在 `sqlite` / `postgres` 两个 dialect package 内分别保留 `repository_room_delete.go`、`repository_room_load.go`；过薄 util helper 已合回主文件。
  - 保留 `sqlite` / `postgres` 两个入口仓储，避免迁移时改变驱动边界。
  - 公共 scanner、conversation 选择和 latest context 选择已移动到 `internal/storage/roomrepo/scan_room.go`，减少重复但不模糊 SQL 差异。
- Workspace history：
  - `store_agent_history.go` 拆为 `store_agent_history.go`、`store_agent_history_overlay.go`、`transcript_locator.go`、`transcript_cache.go`、`transcript_reader.go`、`transcript_projection.go`、`transcript_guidance.go`。
  - 保持 `AgentHistoryStore` 对外 API 不变。
- Workspace input queue：
  - `store_input_queue.go` 已拆为入口、JSONL file、replay、codec、order 同包文件。
  - 保持 `InputQueueStore` 对外 API 不变。
- Workspace history normalize：
  - `history_normalize.go` 已拆为入口、pagination、result summary、external delivery、unfinished round、order 同包文件。
  - 保持历史归一化、分页和 active round 输入函数签名不变。
- Workspace live manager：
  - `manager_live.go` 已拆为订阅生命周期、watcher、write flush、util、diff 同包文件。
  - 保持 `liveManager` 对 Service 的方法集合不变。
- Workspace service：
  - `service.go` 已拆为 Service core、model、agent workspace、file、mutation、upload、path helper 同包文件。
  - 保持 `Service` 对外文件读写、上传、下载和 live 订阅方法签名不变。

### Protocol

当前状态：

- `internal/protocol` 文件命名总体遵守 `model_xxx.go`。
- `model_automation.go` 已收缩为 automation 常量和错误；具体模型拆到 `model_automation_schedule.go`、`model_automation_target.go`、`model_automation_job.go`、`model_automation_report.go`、`model_automation_input.go`、`model_automation_heartbeat.go`。

判断：

- 这些类型确实跨 HTTP/WebSocket/前端/运行时边界，放在 protocol 是合理的。
- 问题是文件边界，不是 package 边界；第一轮同包拆分已解决单文件过大问题，没有改导出类型名、JSON tag 或枚举值。

建议迁移：

- 后续新增 automation 协议时继续放入对应文件，避免 `model_automation.go` 重新变成 catch-all。
- 若 `model_automation_job.go` 继续增长，再按 job、run、status/history 拆第二层同包文件。

### Handler

当前状态：

- 多数 handler 仍是 API binding 层，没有明显业务下沉不足。
- `internal/handler/websocket/handler.go` 已拆成入口、connection、dispatch、app event subscription、room subscription、session binding、control、error、values 等同包文件；主入口为 207 行。

判断：

- handler 层总体可接受。
- WebSocket handler 第一轮结构债已解除；后续新增消息类型应继续落到对应 `handler_*` 分片，避免主入口重新变厚。

建议迁移：

- `handler.go` 保留 Handler 构造、连接入口和 room 广播 API。
- 已新增 `handler_connection.go`、`handler_dispatch.go`、`handler_room_subscription.go`、`handler_session_binding.go`、`handler_control.go`、`handler_error.go`、`handler_values.go`。
- subscription registry 文件当前边界清楚，暂不迁移。

### Entrypoint / App Server / CLI

当前状态：

- `cmd/nexus-server/main.go` 负责配置加载、日志、文件句柄限制、数据库迁移、owner bootstrap、HTTP server 启动和信号生命周期。
- `internal/app/server` 已经按 routes、web routes、lifecycle、service factory、MCP builder、WebSocket builder 拆分，单文件规模健康。
- `cmd/nexusctl` 很薄，CLI 主体在 `internal/cli`。
- `internal/cli` 中 `command_memory.go` 已拆到 51 行入口；`command_automation.go` 仍偏长，但主要是 cobra 命令树、flag binding 和输出整形，结构风险低于 provider/runtime/storage。

判断：

- 入口层没有明显“屎山”。
- `cmd/nexus-server/main.go` 可以长期保持当前规模；只有当 owner bootstrap 或 migration 策略继续增长时，再迁到 `internal/app/server/bootstrap` 一类的内部包。
- CLI 可以按命令组继续自然拆分，不需要优先做架构迁移。

建议迁移：

- 暂不处理入口层。
- CLI 后续只在功能变更时顺手拆，例如按 `command_automation_*` 继续收敛自动化命令入口。
- 不要把 server startup 逻辑下沉到 service 层；当前放在 `cmd` 和 `internal/app/server` 的边界正确。

## 前端结构审计

当前状态：

- `web/src` 已经有清楚的 `app`、`features`、`hooks`、`lib`、`shared`、`store`、`types`、`pages` 分层。
- API client 位于 `lib/api`，类型位于 `types`，共享 UI 位于 `shared/ui`，方向合理。
- 大文件集中在 feature 组件和 hooks：
  - `shared/i18n/messages.ts`：1580 行。
  - `hooks/agent/use-agent-conversation.ts`：1243 行。
  - `features/settings/provider-settings-panel.tsx`：682 行。
  - `features/conversation/shared/composer-panel.tsx`：673 行。
  - `shared/ui/select-menu.tsx`：574 行。
  - `shared/ui/sidebar/sidebar-wide-panel.tsx`：521 行。
  - `features/conversation/shared/editor/editor-panel.tsx`：517 行。

判断：

- 前端不是目录乱，而是局部 feature 文件承担 container、state machine、API orchestration、view、dialog state、format helper 多重职责。
- `use-agent-conversation.ts` 已完成 runtime state、volatile snapshot、history loader、stream buffer 分片；本轮只做同文件 reset helper 收敛，避免再拆出薄 hook。
- `editor-panel.tsx` 已完成 preview model / HTML / media / Office fallback 分片，入口降到 517 行；后续只需继续拆文本编辑加载/保存 hook。
- `composer-panel.tsx` 已完成本地附件、pending queue、@mention 和 footer/action menu 分片，入口降到 673 行；后续只在发送流程或 IME/input history 继续增长时再拆。
- `channels-directory.tsx` 已完成 channel model、guide、login panel、accounts panel 和 card 分片，入口降到 232 行；不再作为大文件优先项。
- `home-sidebar-panel.tsx` 已完成目录 hook、conversation model 和 row 组件分片，入口降到 418 行；后续不再作为大文件优先项。
- `message-item-model.ts` 已完成 stats/result、permission 匹配、assistant ordering 和 activity 摘要分片，入口降到 738 行；后续可继续拆 final assistant projection。

建议迁移：

- 对大 feature 文件采用 `*.types.ts`、`*.model.ts`、`*.controller.ts`、`*.view.tsx`、`components/*` 拆法。
- `use-agent-conversation.ts` 已拆出 runtime state、volatile snapshot、history loader 和 stream buffer；后续只在 websocket event binding 或 message mutation 能形成清楚边界时再拆。
- `editor-panel.tsx` 已拆出 preview 组件和文件类型模型，后续如继续增长再拆 text editor controller。
- `composer-panel.tsx` 已拆出 attachment local model/list 和 pending queue，后续应继续拆 keyboard/input history hook 与 composer footer actions。
- `channels-directory.tsx` 已拆出 model、icon/style、guide、login/accounts/card，剩余 directory state 和 connect dialog 暂可留在入口。
- `home-sidebar-panel.tsx` 已拆出 directory hook、conversation/unread model 和 chat/contact row，剩余容器编排暂可留在入口。
- `message-item-model.ts` 已拆出 stats、permission、ordering、activity helper，剩余 hook 主体继续保留状态编排。
- `provider-settings-panel.tsx` 已完成第一层 UI/model 拆分，后续应继续拆请求 hooks 和保存/测试/删除流程。
- `messages.ts` 可按 namespace 或页面拆分，但要保持 i18n provider 导出兼容。

## Desktop / Deploy / Scripts 审计

当前状态：

- `deploy/data` 和 `desktop/macos/.build` 都已被 `.gitignore` 忽略，没有进入 git 索引。
- `desktop/macos/Sources` 目录结构清楚，按 Update、WebView、Bridge、Window、Diagnostics、Security、Sidecar、Hotkey、Lifecycle、URL 等职责分组。
- Swift 源码最大文件是 `DesktopUpdateChecker.swift`，约 1112 行；其次是 `WebViewHost.swift`，约 577 行。
- `scripts/desktop/fetch-nxs-runtime.js` 约 451 行，`deploy/entrypoint.sh` 约 315 行，属于发布/运行脚本中偏大的文件。

判断：

- desktop 顶层结构没有明显目录问题。
- `DesktopUpdateChecker.swift` 是桌面端明显结构风险，可能混合版本检查、下载、校验、状态上报和安装流程。
- deploy/scripts 目前优先级低于后端核心结构债，但 shell 脚本继续增长时应拆成小函数或 helper 文件。

建议迁移：

- 桌面端后续单独审计 `DesktopUpdateChecker.swift`，优先拆 update manifest、download、verification、installer coordination、UI/status reporting。
- `fetch-nxs-runtime.js` 如果继续承载更多平台逻辑，应拆 platform resolver、download/cache、checksum、CLI output。
- `deploy/entrypoint.sh` 保持启动脚本定位，不下沉复杂业务逻辑。

## 推荐迁移顺序

0. 保留已经完成的第一轮局部结构修复。
   - `connectors`、Feishu Docx markdown render、IM `feishu/wecom_bot/personal_weixin/DingTalk/Telegram`、IM router/ingress/pairing/login、IM ingress envelope tests、automation task/heartbeat/execution/observability、goal progress retry、Provider 服务、Provider storage repository、Runtime manager、Runtime executor round、Runtime debug message、Workspace service、Workspace initializer、Workspace agent history、Workspace input queue、Workspace session file store、Workspace history normalize、Workspace live manager、Workspace memory engine、Conversation titlegen、Room visible context、Room service、Room input queue / execution / chat / round state、WebSocket handler、Room repository、Room repository scanner、Protocol automation、Session service、Auth service、Auth storage repository、Skill marketplace search、Skill CLI command、Memory CLI command、Logx pretty handler、Imagegen service 已完成本地拆分。
   - 后续只需在最终提交前再跑全量测试。

1. Workspace service / storage 大文件复核。
   - 风险：中。
   - 收益：中。
   - 验证：`go test ./internal/service/workspace ./internal/storage/workspace`。

2. 前端大 feature / hook 拆分。
   - 风险：中。
   - 收益：高。
   - 验证：`pnpm --dir web lint`、`pnpm --dir web typecheck`，必要时补局部组件 smoke 测试。

3. CLI 大命令文件机会型拆分。
   - 风险：低。
   - 收益：低到中。
   - 验证：`go test ./internal/cli ./cmd/nexusctl`。

5. Desktop updater 拆分。
   - 风险：中。
   - 收益：中。
   - 验证：macOS desktop build / smoke 脚本。

## 不建议迁移的内容

- 不建议把 `internal/protocol` 的 automation 类型迁出 protocol；这些模型确实跨边界共享。
- 不建议把 `internal/service/room` 拆成多个 package；当前 room 是聚合域，同包拆文件更稳。
- 不建议马上合并 SQLite/PostgreSQL Room repository；双 dialect 直接合并容易引入 SQL 差异回归。
- 不建议把 frontend 的 `features` 目录重命名或整体搬迁；问题在局部大组件，不在顶层目录。
- 不建议在结构审计阶段顺手改协议字段、DB schema 或 public API。
- 不建议把 build 产物、运行数据或 deploy/data 纳入结构重构范围；这些目录应继续保持 git ignore。

## 待补充审计

- [x] Provider 服务结构和模型边界。
- [x] Room / DM / Session 对话域边界。
- [x] Runtime manager / executor / provider / permission 边界。
- [x] Storage repository 双实现与 SQL 复用策略。
- [x] Workspace storage 和 workspace service 分层。
- [x] Protocol 大模型文件拆分规则。
- [x] Handler 层是否存在业务逻辑下沉不足。
- [x] Web 前端结构是否需要单独审计。
