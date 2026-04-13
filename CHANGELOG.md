# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- 新增自动化运行时基础设施：`heartbeat` 主会话轮询、`cron` 精确定时调度、system event queue、wake bookkeeping、统一 delivery router、scheduled task run ledger。
- 新增自动化后端 API：`/agent/v1/automation/heartbeat/*` 与 `/agent/v1/capability/scheduled/tasks*`，支持 heartbeat 状态/唤醒、定时任务 CRUD、立即运行、启停与运行记录查询。
- 新增自动化测试覆盖：`tests/automation/*` 与 `tests/api/test_automation_api.py`，覆盖模型、仓储、投递、运行时、heartbeat、cron 与 API 基本行为。
- 新增主智能体编排侧的定时任务启用/禁用 CLI 命令，以及对应的编排回归测试。
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
- 新增 workspace 文件下载接口，前端可直接获取 Agent 工作区中的原始文件。

### Changed
- 重构 `/app` 与 `launcher` 前端视觉骨架：收口 surface recipe、统一目录卡片语法、移除多余玻璃壳与中间 wrapper，整体层级更薄、更接近桌面端结构。
- Agent Options 全面接入 i18n：导航、标题、身份/设定/技能/工具页文案统一走国际化字典，工具预授权说明补齐中英文描述，并明确默认语言兜底为中文、首次加载优先遵循浏览器语言。
- 简化 Launcher Hero、右侧 Nexus 信息面板与输入/动作区结构，恢复 `/app` 首页原有 `nexus` ASCII 文字动画入口。
- 主智能体编排 CLI 的 `create_scheduled_task` 不再局限于绑定已有会话；当前创建入口已对齐为结构化会话目标，支持 `main / bound / named / isolated`，并提供 `--main`、`--isolated`、`--session-key`、`--named-session-key`、可选 `--session-target-kind` 与 `--wake-mode` 参数组合。
- Heartbeat 后端接口语义补齐为 `GET /agent/v1/automation/heartbeat/{agent_id}` 读取状态、`PUT /agent/v1/automation/heartbeat/{agent_id}` 更新配置、`POST /agent/v1/automation/heartbeat/{agent_id}/wake` 手动唤醒。
- 定时任务控制台补齐完整操作闭环：支持创建、编辑、删除、立即运行、启停切换、运行历史查看，并在创建/编辑表单中提供真实的 `main / bound / named / isolated` 会话目标选项。
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
- Room workspace 面板升级为可创建文件/文件夹、右键删除与重命名，并对文本、PDF、图片和二进制文件分别提供编辑、预览或下载入口。

### Fixed
- 修复 Sidebar、Room/DM、技能/连接器目录、Agent Options 与 Launcher 等区域因额外背景板、空壳节点和过多动作层导致的布局抖动、容器收缩与层级堆叠问题。
- 修复多处前端无效交互入口与显示回归，包括 Launcher 右侧“清空 Nexus 对话”、连接器卡片状态表达、Thread 面板宽度与会话切换下拉等问题。
- 修复主智能体创建定时任务时未校验 `session_key` 归属的问题，避免把任务错误绑定到其他 Agent 的会话。
- 修复 `nexus-manager` skill 中单次定时任务示例使用过去时间的问题，并补充 `at` 模式只会触发未来时间的说明。
- 修复 `session_repository` / `cost_repository` 模块级初始化产生的导入副作用（#11）。
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
- 新增基于 workspace 文件存储的成本账本能力。
- 新增 Session / Agent 成本汇总 API。
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
