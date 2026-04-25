# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 个人设置页新增头像设置，复用 Agent 头像资源，并将头像同步到个人资料与登录状态。

### Fixed
- 修复 Docker 构建时 Corepack 通过 npmmirror 拉取 pnpm 元数据返回 404 的问题，改为用 npm 安装固定版本 pnpm。
- 修复 usage 落账遇到 SDK JSON 数字类型时被判空，导致设置页 token 用量无数据的问题。
- 修复个人头像未显示在 DM、Room 主消息区和 Room Thread 用户消息上的问题，并确保头像变更会触发消息项重渲染。
- 修复 Room 中被无需回复标记过滤的轮次没有写入 token usage 台账的问题。

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
