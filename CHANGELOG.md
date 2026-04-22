# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- :sparkles: Connector 端到端落地：支持 Web 自助配置 OAuth 应用；已连接 connector 以 `nexus_connectors` MCP server 形式对 Agent 暴露 `connector_list` / `connector_call` 工具。

### Changed
- Connector 目录暂只保留 `github` 作为 available，其余 provider 置为 coming_soon；OAuth 配置弹窗在 GitHub connector 下补充"去 GitHub 创建 OAuth App"帮助链接，自动带上当前环境的 callback URL。
- 收口 chat / room 运行时骨架到 `conversation` 层：`RoundCoordinator` 统一处理 round 生命周期、runtime client 复用与会话状态广播，`Dispatcher` 统一 DM / Room / ingress / automation / gateway websocket 的入站路由。
- 重构 `internal/gateway` 结构：共享中间件与响应能力下沉到 `gateway/shared`，HTTP handlers 按 `auth / core / agent / workspace / room / launcher / skill / connector / channel / automation / capability` 分域拆包，根包仅保留 server wiring、routes、websocket 生命周期与 subscription registry。
- 统一 Connector OAuth 回调路径到 `/capability/connectors/oauth/callback`，需要在各 provider 后台重新登记。
- 多用户模式改为真正的 `user_id` 级隔离：`agent / room / session / workspace` 全链路按当前登录用户收口，主智能体改为按用户作用域初始化；共享 `ACCESS_TOKEN` 兼容入口只在系统尚未初始化用户时可用，避免绕过多用户隔离。
- 重构 `nexus_automation` MCP 工具入参体系，全面对齐 UI「新建任务」对话框并放宽创建门槛：
  - **结构化字段替代原始 cron**：`schedule.kind` 提供 `single / daily / interval` 三种 UI 对齐形态（`daily_time + weekdays`、`interval_value + interval_unit`），同时新增 `cron` kind 让熟悉标准 5 段表达式的 agent/用户可以直接传 `expr`（别名 `cron` / `cron_expression`）。
  - **UI 语义收口**：移除工具层对底层 `session_target / delivery / source` 对象的直通入口，统一通过 `execution_mode(main|existing|temporary|dedicated)` + `reply_mode(none|execution|selected)` 表达；`execution_mode=main` 仅主智能体可用，普通 Agent 调用会被拒绝。
  - **Agent scope**：普通 Agent 只能 CRUD 自己 `agent_id` 名下的任务，`list_scheduled_tasks` 默认只返回自己的任务；主智能体（`config.DefaultAgentID`）豁免。
  - **Lenient 默认**：短文本（≤24 字、无"总结/汇总/报告/summary/report/analyze"等中英文重业务关键词）提醒类任务可省略 `execution_mode` / `reply_mode`，工具会默认按 `temporary + none` 创建；`schedule.timezone` 缺省回退 `config.DefaultTimezone`（默认 `Asia/Shanghai`）。
  - **扁平字段兼容**：顶层平铺的 schedule 字段（`kind` / `run_at` / `daily_time` / `expr` 等）会自动重组为嵌套 `schedule` 对象，兼容不喜欢嵌套的模型。
  - **cron 回翻成 UI 可编辑形态**：agent 经 `kind=cron` 提交的任务，工具层会把表达式翻译回 `daily_time + weekdays` 语义（要求 minute/hour 为单整数、dom/month=`*`）；无法翻译的表达式直接拒绝，避免产生 UI 无法编辑的「幽灵任务」。
  - Skill `scheduled-task-manager` 与默认/主智能体 AGENTS 模板同步更新，明确「ScheduleWakeup 仅用于会话内自我提醒，所有用户可见的定时需求都走 `create_scheduled_task`」的分流规则与自管原则。

### Added
- 新增 workspace live 订阅链路：`subscribe_workspace / unsubscribe_workspace` 现已接通 Go 端文件事件与 `agent_runtime_event` 实时广播，前端可直接消费 `workspace_event` 与运行态快照。
- 新增 Discord / Telegram 真入口适配器：Go 后端启动后可直接接收外部通道消息并统一路由到现有 `session_key + EventMessage` 聊天主链。
- 新增 Go 后端双仓库骨架：主仓库引入 `cmd/nexus-server`、`cmd/nexus-migrate`、`cmd/nexusctl`、`cmd/protocol-tsgen`，并建立 `internal/gateway`、`internal/protocol`、`internal/runtime`、`internal/chat`、`internal/room`、`internal/permission`、`internal/storage`、`internal/automation`、`internal/channels`、`internal/skills`、`internal/connectors`、`internal/workspace` 与 `internal/cli` 分层骨架。
- 新增独立 `nexus-agent-sdk-go` 仓库骨架，承载 Claude Code/Claude CLI agent core 封装、消息类型、client 生命周期与示例程序。
- 新增 Goose schema migration 基础设施：`db/migrations/sqlite`、`db/migrations/postgres`、`sqlc.yaml` 与双数据库初始 schema。
- 新增 Go 端协议生成链路：`cmd/protocol-tsgen` 与 `web/src/types/generated/protocol.ts`，开始将前端共享类型的真相源收敛到 Go 协议层。
- 新增自动化运行时基础设施：`heartbeat` 主会话轮询、`cron` 精确定时调度、system event queue、wake bookkeeping、统一 delivery router、scheduled task run ledger。
- 新增自动化后端 API：`/agent/v1/automation/heartbeat/*` 与 `/agent/v1/capability/scheduled/tasks*`，支持 heartbeat 状态/唤醒、定时任务 CRUD、立即运行、启停与运行记录查询。
- 新增自动化测试覆盖：`tests/automation/*` 与 `tests/api/test_automation_api.py`，覆盖模型、仓储、投递、运行时、heartbeat、cron 与 API 基本行为。
- 新增定时任务控制台：Heartbeat 状态卡片、定时任务列表、结构化创建对话框、运行历史弹窗，以及侧栏实时任务数。
- 新增 `capability` 能力模块，统一管理技能市场、连接器、定时任务、渠道与配对能力。
- 技能系统数据持久化：新增 `pool_skills` / `agent_skills` 数据库表与 Alembic 迁移，技能安装状态、全局开关、Agent-Skill 关联全部写入 SQLite，取代旧 JSON 文件存储。
- 新增 `SkillSqlRepository` 与 `SkillRepository` 数据访问层。
- 新增浏览器密码登录能力：后端签发 `HttpOnly` 会话 Cookie，前端新增登录页、路由守卫与退出登录入口，HTTP / WebSocket 统一受登录态保护。
- 新增服务端会话持久化：登录令牌 SHA-256 摘要存入 `auth_sessions` 表，支持即时撤销；移除 `AUTH_SESSION_SECRET` 配置项。
- 新增 `PermissionDispatchRouter`：Room 权限请求优先走 `subscribe_room` 广播通道，DM 走 sender 直投。
- 新增 Room 重连恢复：`subscribe_room` 后从 DB 查询活跃 slot 补发 `chat_ack`，slot 携带真实 `round_id` / `status` / `timestamp`。
- 新增 `MessageActivityStatus` 组件：按当前工具阶段展示 thinking / browsing / executing / waiting_permission / waiting_input 五种状态，替代旧 `MessageLoadingDots`。
- 新增 `request_api` 统一 HTTP 客户端：自动 `credentials: "include"`、401 事件广播、JSON 响应校验，全部 API 文件从 raw `fetch` 迁移。
- 新增 `AgentConversationRuntimeMachine` 状态机：集中管理 pending/active round、message tracker、permission count，`snapshot()` 计算 `phase` 与 `is_loading`。
- 新增 `AgentConversationIdentity` 统一身份对象，替代散落的 `agent_id` / `room_id` / `conversation_id` / `chat_type` 四元组。
- 新增 Docker 部署配置：单阶段 Dockerfile、nginx 网关（WS 代理 + SPA 路由）、docker-compose（nexus + nginx 双服务 + 健康检查）。
- 新增 Vite dev proxy：`/agent` 代理到 `127.0.0.1:8010`，含 WebSocket 支持。
- 新增 main agent 编排 CLI（`agent/cli.py`），支持 `list_agents`、`create_agent`、`validate_agent_name`、`list_rooms`、`create_room`、`add_room_member` 命令。
- 新增 main agent 编排服务层，对外暴露多 Agent 与 Room 管理能力。
- 新增 `nexus-manager` skill，规范化 YAML frontmatter、命令参考、Workspace 规则与操作流程文档。
- 首页 App 对话交互完整打通：双态过渡、消息收发、权限决策、中断/重试、创建协作直达首条对话。
- 支持 `AskUserQuestion` 自定义回答选项。
- 新增 `room conversation` CRUD API，并接通 room 页面真实的新建/删除对话、重命名 room、增删成员与删除 room 管理能力。
- 新增同一 session 的“多观察者、单控制者”运行时语义：多窗口可同时实时观察同一会话，消息流与 round 状态 fan-out，同一时刻仅一个控制端可发送消息、停止生成或确认权限。

### Changed
- DM 会话历史切换为 `cc transcript + Nexus overlay` 双层结构：新建 DM session 默认使用 transcript 作为历史真相源，Nexus 只补 `round marker` 与 synthetic overlay，Session API 不再回退旧 `messages.jsonl`。
- Room 历史链继续收口：成员私有 session 不再双写完整 `messages.jsonl`，共享 room 历史也不再保存完整正文副本，统一改为 `inline overlay + transcript_ref` 共享索引层，真正正文按需从对应成员 transcript 投影恢复。
- 运行时正式停用 legacy session 正文链路：DM、delivery 与 Session API 现在统一要求 `history_source=transcript`，旧 `messages.jsonl` 不再属于受支持真相源。
- 历史真相源规则进一步收口：`assistant` 正文与 `usage` 只来自 `cc transcript`，`result` 只来自 Nexus overlay；运行时读取 transcript 时不再投影 `MessageTypeResult`，迁移器也不再把 legacy result 转成 `transcript_ref`。
- GitHub Release 发布 workflow 正式切换到 Go 后端链路：移除旧 Python 依赖安装，改为 `actions/setup-go` + `go mod download`，避免发布流程仍停留在 Python 时代。
- session / room 文件目录命名统一收口为可读语义路径：DM 使用 `dm-<channel>-<ref>`，Room 私有与共享使用 `room-<conversation_id>`；运行时不再兼容旧 base64 与短名+hash 目录布局。
- 移除 `nexusctl session migrate-history` 与对应 legacy 迁移代码：历史迁移不再作为运行时或 CLI 能力保留，仓库主链只维护当前 `transcript + overlay / transcript_ref` 结构。
- workspace 文件存储根现在统一跟随 `NEXUS_CONFIG_DIR / CLAUDE_CONFIG_DIR`，Room 共享 overlay、transcript 与迁移命令不再偷偷回落到真实 `~/.nexus`。
- Go 后端 `list_agents` / `get_agent` 现在补齐 `skills_count`，并与 Python 主线对齐；DM session 复用 SDK client 时会自动执行 `Reconfigure`，`model / max_thinking_tokens / max_turns / setting_sources` 等配置会在下一轮尽可能热更新，涉及工作区、工具白名单和运行时环境的变更则自动带 `resume` 重连。
- Goose 迁移历史收口为 `00001` Python 最终基线 + `00002` Go 适配迁移，`cmd/nexus-migrate` 会在启动前识别当前数据库属于 Python 最终结构还是 Go 当前结构，并把 Goose 版本对齐到正确阶段，避免运行期继续撞旧认证域结构。
- 前端默认 Agent 模型 fallback 与部署默认值统一切换为 `glm-5.1`。
- `make dev`、`make run-backend`、`make check`、`deploy/Dockerfile`、`deploy/docker-compose.yml` 默认切换为 Go 后端链路；Python 入口降级为 `make run-backend-python` / `make dev-python` 兼容命令。
- Docker 后端镜像改为 Go 多阶段构建，并通过 vendored SDK 依赖消除本地绝对路径 `replace` 对容器构建的阻塞。
- `make db-init` 迁移到 Goose 执行链路，并新增 `dev-go`、`run-backend-go`、`check-go`、`gen-protocol-types` 目标，为 Go 服务与协议生成提供基础工作流。
- 定时任务前后端契约升级为结构化自动化模型：前端不再使用扁平 `cron_expression/source_type` 占位字段，统一改为 `schedule`、`session_target`、`delivery` 结构。
- 技能市场代码从 `service/workspace/` 迁移至 `service/capability/skills/`，API 从 `api/agent/` 迁移至 `api/capability/`。
- `SkillCatalog` 改为无状态设计，状态由调用方通过数据库查询后传入。
- `SkillService` 全面改用 `skill_repository` 进行状态读写。
- DM / Room 会话入口与运行态统一收口到同一条 room-based identity + runtime machine 链路，历史 / 工作区 / 简介切换不再丢失顶部 header 或执行上下文。
- 聊天中的运行反馈统一为状态条，按 `thinking / replying / browsing / executing / waiting_permission / waiting_input` 展示，并接入 `unicode-animations` 动效。
- 优化 Skills 页面交互与布局：搜索置顶、分类降级为轻筛选、操作反馈改为自动消失的轻量提示，能力侧栏改为总览卡片并显示全局已安装能力数量。
- capability 后端正式接入主路由，技能市场与连接器读写统一走新的 SQLite repository 与 capability service。
- Skill catalog 自动补齐本地可发现但未手工编目的 builtin skills，避免能力页内容缺漏。
- 更新 `makefile`，新增 `db-init` / `check` / `lint-web` / `typecheck-web` / `check-backend` 目标，并在本地启动后端前自动执行迁移。
- 主智能体 ID 与名称统一为 `nexus`，移除 `main` 硬编码，后端通过 `settings.DEFAULT_AGENT_ID` 统一引用，前端新增 `DEFAULT_AGENT_ID` 常量。
- 前端品牌文案从"真格 App"更新为"Nexus"。
- `persistence` 包重命名为 `repository`（`agent/service/persistence` → `agent/service/repository`，`agent/api/persistence` → `agent/api/repository`）。
- 前端配置文件合并 `options.ts` + `runtime-config.ts` 为统一的 `options.ts`。
- 前端 URL 解析简化为纯同源相对路径，开发环境由 Vite proxy 转发，移除 localhost 检测与端口自动路由逻辑。
- CORS 配置禁止 `*` 通配符，改为显式白名单（默认 `localhost:3000` / `4173`）。
- 首页 `Nexus` 侧边面板收敛为单一 chat 入口，移除 `Workspace / About / 推荐动作` 等非必要工作区结构，减少首页干扰。
- 扩展 `agent/cli.py` 与 `nexus-manager` skill，新增 agent / room / workspace / skill 系统操作指令，支持 Nexus 直接完成更多平台管理动作。
- `fail()` 不再修改共享 Resp 模板，改为构建独立 payload；4xx 响应使用 `logger.warning` 而非 `error`。
- Loading 态从 `useState` + 手动 set 改为运行时状态机派生，移除所有 `set_is_loading` 调用。
- Room Page Controller 拆分为三层：`core` 纯函数 + `data` hook + `agent dialog` hook。
- Conversation Store 移除 `current_session_key`、CRUD actions 与 `utils`，会话管理上升到页面层。
- 前端 sidebar 字号微调（10→11、11→12、12→13），Launcher placeholder 精简。
- Launcher 页面控制器移除 `use-home-agent-conversation-controller` 和 `use-initialize-conversations`，直接使用 store + agent dialog hook。
- WebSocket session 绑定从“单活 sender”升级为“多绑定 + 单控制端”，`bind_session` 新增 `client_id / request_control` 语义，`session_status` 同步控制端与观察者数量。
- Room 权限请求不再广播给全部房间订阅者，改为只投递给当前 session 控制端；Room 普通协作事件仍保留 room 广播。

### Fixed
- 修复多用户部署下 `nexus-manager` 通过 `nexusctl` 执行系统操作时丢失当前登录用户作用域的问题：CLI 业务命令现在强制走请求上下文/`NEXUSCTL_USER_ID` 作用域，agent runtime 会按当前登录用户注入该环境变量，避免跨用户读取或操作 `agent / room / session / workspace / skill` 数据。
- 修复 `session_repository` 模块级初始化产生的导入副作用（#11）。
- 修复 Alembic 迁移多 head 冲突问题。
- 修复 `make dev` / `make db-init` 因 Alembic 双 `head` 与后端启动旧导入路径导致的本地启动失败问题。
- 修复 DM / Room 在权限确认、工具执行、停止生成、AskUserQuestion 回答后等场景下输入框提前解锁、状态闪断、确认卡片丢失与光标/状态提示错位的问题。
- 修复 `AskUserQuestion` 多选字段兼容问题，前端现在同时支持 `multi_select` 与 `multiSelect`。
- 修复 Room / DM 删除会话时遗漏 workspace session 目录清理，以及历史页最后一个 / 主对话删除按钮缺失、说明提示被裁切的问题。
- 修复 `web` 依赖安装失败：移除失效且未使用的 `@anthropic-ai/claude-code` 遗留依赖，并增加项目级 npm registry 配置。
- 修复技能市场与能力侧栏的多处状态不一致问题，包括全局开关/更新不同步、Agent 技能配置保存不生效、详情弹窗安装状态误显示，以及 DM 跳转后侧栏高亮错误。
- 修复首页主对话长内容溢出与自动贴底滚动问题。
- 修复 `AskUserQuestion` 回答后被中断的问题。
- 修复 Room 刷新后半截历史被误判为"仍在协作"：Room 的活跃态只信 WS 事件，不根据历史消息推断。
- 修复 Room `session_status.is_generating=false` 时前端残留态未收口。
- 修复 `getAgentRoundStatus` 在无 pending slot 时将已结束的 assistant 误判为 streaming。
- 修复 `skill_import_service` Zip Slip 路径遍历安全漏洞（#45）。
- 修复 Token 比较使用 `!=` 的时序侧信道，改用 `hmac.compare_digest`（#46）。
- 修复异常处理器直接修改 Resp 全局单例导致的并发安全问题（#28, #36）。
- 修复 `Settings.__str__()` 未脱敏敏感配置（#38）。
- 修复 `repeat_even` 异常处理中 `except as e` 与后续 `exc` 引用不一致的 `NameError`（#34）。
- 修复 bare except 替换为 `except Exception` 并补充 `exc_info`（#37）。
- 修复定时任务类型缺失导致类型检查失败。
- 修正 `credentials` 字段误导注释（#25）。
- 移除 `requirements.txt` 中重复的 `alembic>=1.14.0`（#29）。
- 移除与 `requires-python>=3.11` 矛盾的 Python 3.10 classifier（#30）。
- 修复同一会话多窗口场景下最后绑定窗口独占实时流、权限卡重复弹出、非主窗口仍可误发 `chat / interrupt / permission_response` 的问题。

### Removed
- 删除 `backfill_service` 旧数据回填服务及全部回填调用链。
- 删除 `legacy_sync_bridge` 旧模型到新数据库的桥接模块。
- 删除 `migrate_workspace_runtime_layout` 工作区布局迁移逻辑及 6 处调用。
- 删除成本账本从 `messages.jsonl` 回填重建的防御逻辑。
- 删除 `cache_creation_tokens` 旧字段名兜底、ISO 时间戳双格式解析。
- 删除 `CreateAgentRequest.workspace_path` 兼容字段，workspace 路径完全由后端管理。
- 删除 `legacy_db_path` 死属性、`LEGACY_MAIN_AGENT_SKILL_NAMES` 空操作。
- 删除 `agent_service` 中向 SQL 双写同步的 `ensure_main_agent_ready` / `_sync_agent_to_sql` 路径。
- 删除 `session_store` 中向 SQL 双写同步的 `_sync_session_to_sql` / `_sync_message_to_sql` 路径。
- 删除 `StreamingCursor` / `InlineStreamingCursor` 组件，由 `MessageActivityStatus` 替代。
- 删除 `use-home-agent-conversation-controller`、`use-initialize-conversations`、`store/conversation/utils`。

### Docs
- 精简并重写 `session-key / permission-runtime / main-agent / message-processing / skill / room / frontend-design` 七份 spec，统一到当前 Go 后端、`transcript + overlay` 历史真相源与 round 分页实现，移除过时的 Python/legacy 叙事。
- 更新根目录 `README.md`，同步当前 React + Vite 前端、混合持久化、Memory、Room/DM 页面结构与配置说明。
- 更新 `web/README.md`，修正前端技术栈、目录结构、路由与 `VITE_*` 环境变量说明。
- 更新 `env.example` 与部署说明，补充公网部署场景的 `AUTH_LOGIN_*` / Cookie / `BACKEND_CORS_ORIGINS` 配置项。
- 更新 `docs/message-processing-spec.md`、`docs/permission-runtime-spec.md`、`docs/room-spec.md` 补充 Room slot 恢复与权限派发规范。
- 更新 `docs/permission-runtime-spec.md`，补充多观察者 / 单控制者、控制权抢占、`bind_session` 新字段与 `session_status` 扩展约束。

## [0.0.3] - 2026-03-18

### Fixed
- 修复 Markdown 有序列表在消息区渲染时编号与正文被拆成两行的问题，`1.` 之后不再出现异常换行。

### Changed
- 统一前端主界面视觉风格，聊天工作区、侧栏、状态栏、输入区和空状态切换为同一套软拟态设计语言。
- 统一消息内部块样式，`thinking`、工具执行块、问答块、代码块与消息统计条改为同心圆角和一致的面板层级。
- 统一配置与确认流程弹窗样式，`AgentOptions`、权限确认和确认/输入对话框与主界面视觉保持一致。
- 收敛任务浮层与 Markdown 表格等剩余组件的圆角、描边与阴影节奏，减少界面风格割裂。
- 新增 `Agent / Profile / Runtime / Room / Conversation / Session` SQLite ORM 模型与 Alembic 初始迁移，建立新的站内协作数据骨架。

## [0.0.2] - 2026-03-17

### Fixed
- 修复删除 Agent 时仅归档记录、不回收工作区目录与活跃 session 的问题，删除后不再残留旧 workspace。
- 修复 `thinking` 在后续 assistant 快照到达后被覆盖消失的问题，思考块现在会稳定保留在同一轮消息中。
- 修复 `tool_result` 被拆成独立 assistant 气泡的问题，工具结果现在会回填到对应 assistant 段内展示。

### Changed
- 重写后端消息处理器，按 SDK 实际消息节奏收口为更薄的 `ChatMessageProcessor + AssistantSegment + SdkMessageMapper` 结构。
- 收紧前端流式边界：只有 `thinking / text` 参与 `StreamMessage` 增量渲染，工具调用与工具结果统一走完整消息快照。


## [0.0.1] - 2026-03-14

### Fixed
- 修复 `thinking` / 文本流式内容在前端被二次打字机动画延迟显示的问题，恢复按后端真实 chunk 即时渲染。
- 修复消息流式链路中 assistant 段收束、工具结果插入与同一 `message_id` 更新顺序不稳定的问题。
- 修复 `TodoWrite` 提取、会话删除与工作区侧栏在空块/空 `session_key` 场景下的前端异常。

### Changed
- 重构消息协议边界，新增 `StreamMessage`，统一后端流式消息、最终消息与前端消费模型。
- 调整 WebSocket / IM 发送层，显式区分 `message`、`stream` 与 `event` 三类传输。
- 默认向 SDK 透传 `include_partial_messages`，并同步收敛前端无效的流式/轮次配置项。

## Legacy

## 2026-03-13

### Docs
- 新增 `docs/nexus-technical-doc.md`，补充项目技术架构、核心链路与技术交流提纲。

## 2026-03-12

### Changed
- 将 `agent/core/` 重命名为 `agent/config/`，更准确反映模块职责。
- 移除未使用的 `agent/cli.py` 及相关 Redis、数据库模块。

## 2026-03-11

### Fixed
- 修复 Safari 中文输入法导致的 Enter 键误发送问题，使用 ref 替代 state 跟踪输入法状态。
- 修复前端切换 session 时的消息串流错乱：WebSocket 消息现在会严格校验当前激活的 `sessionKey`。
- 修复左侧 session 区域 `msgs` 显示为 0 的问题。
- 修复导入错误和 Next.js 构建警告。
- 修复 Agent Space 三栏间距不一致问题。
- 进一步收敛 Workspace 初始化热路径，减少日志噪音。

### Changed
- 统一 Agent Space 三栏宽度。
- 统一 Workspace Sidebar 与 Agent Inspector 字体大小体系。
- 新增 workspace 实时文件事件链路，支持文件写入实时同步。
- 移除未使用的 SessionRail、session-search 组件。
- 重组组件目录结构，按功能域划分。
- 优化 Agent Space 布局，文件编辑器改为按需展开。
- 接入 Claude SDK 权限决策链路，支持 `permission_suggestions` 与授权决策回传。
- 统一环境模板命名：根目录使用 `env.example`。
- 拆分编辑面板与 workspace 布局，优化过渡动画。

## 2026-03-10

### Fixed
- 修复后端 `session_manager` 导入缺失问题。
- 修复 HTML 嵌套 button 验证错误，改为语义化结构。
- 修复成本账本始终为 0 的问题：补齐 SDK `ResultMessage` 的 dataclass 解析，修正消息保存链路中的 `agent_id` 归一化。
- 修复前端 store 在静态构建阶段直接访问 `localStorage` 的问题。
- 修复 workspace 文件编辑面板展开/收起的跳动。
- 修复左侧会话消息统计。

### Added
- 新增自定义对话框组件 (ConfirmDialog, PromptDialog)，替代 window.confirm/prompt。
- 添加暗黑模式支持 (prefers-color-scheme: dark)。
- 添加全局设计令牌 (success, warning, scrollbar 变量)。
- 新增权限运行时辅助层。
- 增加 workspace 实时文件事件链路。

### Accessibility
- 为所有交互按钮添加 aria-label 属性。
- 添加 focus-visible 聚焦样式。
- 为进度条添加 ARIA progressbar 属性。
- 使用语义化颜色变量 (text-destructive, text-success, text-warning)。

### Changed
- Web 首页重构为 B 端控制台骨架，采用 `Agent Directory -> Agent Space` 的两层信息架构。
- 新增项目级 `SYSTEM_PROMPT.md` 基础提示词层。
- Agent Space 重构为三栏布局：左侧 `Virtual filesystem + Context`，中间 `Session Space`，右侧 `Agent State`。
- workspace 左栏升级为目录树视图，支持文件/目录创建、删除、重命名。
- Agent 内部运行文件统一下沉到 `.agent/` 隐藏目录。
- 前端新增 `browser-storage` 辅助。
- 更换全局字体为 PT Mono。
- 移除未使用的 SessionRail、session-search 组件。

## 2026-03-09

### Changed
- 收口 Session 边界：Session API 不再承载执行配置，执行参数统一归 Agent 管理。
- 默认 Agent 路由改为配置化，通过 `DEFAULT_AGENT_ID` 显式控制默认路由策略。
- Agent 更新配置或 workspace 后，会主动失效内存中的活跃 SDK session。

### Docs
- 在仓库规则中补充"一个完整需求对应一个提交"和"必要时同步更新 changelog"的协作约定。
