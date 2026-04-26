# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- DM 与 Room 输入区新增待发送队列：运行中或已有排队消息时，Enter 会把新输入加入队列，队列项支持手动引导、删除与拖拽排序。
- 常规设置新增用户级默认消息行为与新建 Agent 默认权限模式，偏好写入 workspace JSON，不新增数据库表。

### Changed
- 输入框不再内联展示“排队 / 引导 / 打断”选择，消息行为改由常规设置统一控制，默认行为为排队。
- 常规设置页按外观、常规、权限分区展示，文案与控件更紧凑；偏好项选择后即时保存，权限设置收敛为四种权限模式下拉选择。
- Room 公区消息命中正在回复的 Agent 时不再强制中断该 Agent；忙碌目标会通过 SDK streaming input 接收补充上下文，空闲目标仍正常启动新 round。
- DM 回复中也可继续输入补充要求，新消息会排入当前流式会话，不再默认杀掉正在执行的任务。

## [0.1.1] - 2026-04-25

### Added
- Room 公区协作机制收敛：新增 `room-collaboration` system skill、公区 @ 提及唤醒、Agent 公区发言后的后续 @ 触发，以及无需回复标记输出过滤。
- 个人设置页新增头像设置，复用 Agent 头像资源，并将头像同步到个人资料与登录状态。

### Changed
- 前端与 Docker 部署链路切换到 pnpm：新增 `pnpm-lock.yaml`，移除 `package-lock.json`，并同步更新 makefile、Web 构建镜像、运行镜像与容器内工具链 registry 配置。
- Room 公区上下文改为只向 Agent 注入公区用户消息和其他 Agent 的最终公开结果，不再把工具调用、thinking、tool result 等中间过程塞进其他成员上下文。
- Room 输入区恢复为“只限制正在回复的目标 Agent”，其他 Agent 回复中仍可继续发送普通消息；Room Thread 面板不再因结果消息到达自动关闭。
- Agent 改名允许仅大小写变化的合法更新，同时继续阻止真正重复的名称。

### Fixed
- 修复 Docker 多阶段构建并发复用 apt cache 时可能抢占 `/var/cache/apt/archives/lock` 导致安装失败的问题。
- 修复 Docker 构建时 Corepack 通过 npmmirror 拉取 pnpm 元数据返回 404 的问题，改为用 npm 安装固定版本 pnpm。
- 修复 usage 落账遇到 SDK JSON 数字类型时被判空，导致设置页 token 用量无数据的问题。
- 修复个人头像未显示在 DM、Room 主消息区和 Room Thread 用户消息上的问题，并确保头像变更会触发消息项重渲染。
- 修复 Room 中被无需回复标记过滤的轮次没有写入 token usage 台账的问题。
- 修复 Room 公区上下文注入缺少部分成员公开结果、以及中间过程误进入其他 Agent 输入的问题。
- 修复 Room 新公区消息按 shared session 中断整轮的问题，改为只停止被再次点名的目标 Agent。
- 修复 Room 主动中断后 SDK 流提前关闭被误判为 `round stream closed before terminal` 错误的问题。

## [0.1.0] - 2026-04-24

### Added
- Go 后端主线正式落地：新增 `nexus-server`、`nexus-migrate`、`nexusctl`、协议生成、Goose 迁移，以及 `gateway / protocol / runtime / chat / room / session / workspace / skills / connectors / automation` 分层。
- 浏览器登录与多用户体系落地：支持 HttpOnly Cookie 会话、服务端 session 撤销、用户级主智能体、workspace、room、session 与 skill/connector 数据隔离。
- DM / Room 对话链路升级：接入 `transcript + overlay / transcript_ref` 历史真相源、共享 round 执行内核、多观察者单控制端、Room 重连恢复与权限定向派发。
- Capability 能力面成型：技能市场持久化、结构化定时任务 API/UI/MCP 工具、heartbeat/cron 自动化运行时、GitHub Connector OAuth 自助配置与 `nexus_connectors` MCP 工具。
- Workspace 与外部入口扩展：新增 workspace live 订阅、文件资源块展示、Discord / Telegram 通道入口，以及 Agents / Contacts / Rooms / Settings / Scheduled Tasks / Connectors 等主界面能力。
- 部署链路升级：新增 Go 多阶段 Docker 镜像、nginx 网关、生产健康检查、GitHub Release workflow、运行镜像内置 Agent 工具链与 Docker 管理员自举。

### Changed
- 默认开发、构建、迁移、校验与发布链路切换到 Go 后端；`make dev`、`make db-init`、`make check`、Docker 与 release workflow 均围绕当前 Go 主线运行。
- 网关与业务结构全面收口：HTTP handlers 按领域拆包，共享中间件下沉到 `gateway/shared`，DM / Room / ingress / automation / websocket 入站路由统一由 `Dispatcher` 协调。
- 会话与历史模型收口：运行时不再依赖 legacy `messages.jsonl` 正文链路，session / room 目录统一为可读语义路径，历史读取以 Claude transcript 与 Nexus overlay 为边界。
- `nexusctl` 改为 Agent 友好协议：新增全局 `--json` / `--pretty` / `--verbose`，stdout/stderr 职责分离，成功与错误结构统一，并支持 `--password-stdin`。
- 前端应用结构重组：统一同源 API 客户端、WebSocket 绑定语义、conversation identity、运行态状态机、页面级 controller 与更完整的 onboarding/help 入口。
- 自动化工具参数对齐 UI：`schedule`、`execution_mode`、`reply_mode`、agent scope、cron 回翻与 lenient 默认值统一到可编辑、可审计的任务模型。
- 文档同步到当前架构：README、env 示例、部署说明，以及 session key、permission runtime、main agent、message processing、skill、room、frontend design 等 spec 已精简重写。

### Fixed
- 修复 runtime client 失效、provider/model 热更新、`bypassPermissions` 权限处理、工具参数错误展示、文件路径展示、SDK 依赖预检与 Docker 内 skill 根目录解析问题。
- 修复 DM / Room 在权限确认、停止生成、AskUserQuestion、多窗口观察、重连恢复、活跃态判断与输入框状态上的多处不一致。
- 修复多用户部署下 `nexus-manager` / `nexusctl` 作用域丢失，避免跨用户读取或操作 agent、room、session、workspace、skill 数据。
- 修复本地迁移、Alembic 多 head、旧认证域结构、Go 迁移识别、前端依赖安装与 release workflow 仍引用旧 Python 链路的问题。
- 修复安全与并发问题：Zip Slip 路径遍历、Token 时序侧信道、敏感配置脱敏、Resp 全局单例突变、裸 `except` 与异常变量引用错误。

### Removed
- 删除旧 Python 运行链路、legacy sync/backfill、历史迁移 CLI、旧 workspace runtime layout 迁移、成本账本回填和多处旧字段兼容逻辑。
- 删除 `messages.jsonl` 作为运行时正文真相源的路径，移除旧 session 双写、旧 base64/短 hash 目录布局和旧 result 投影迁移。
- 删除前端旧 conversation store、home conversation controller、loading 手动状态、旧 StreamingCursor 组件与过时的 Session/Workspace 辅助结构。

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
