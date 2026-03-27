# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Docs
- 更新根目录 `README.md`，同步当前 React + Vite 前端、混合持久化、Memory、Room/DM 页面结构与配置说明。
- 更新 `web/README.md`，修正前端技术栈、目录结构、路由与 `VITE_*` 环境变量说明。

### Changed
- 更新 `makefile`，新增 `db-init` / `check` / `lint-web` / `typecheck-web` / `check-backend` 目标，并在本地启动后端前自动执行迁移。
- 主智能体 ID 与名称统一为 `nexus`，移除 `main` 硬编码，后端通过 `settings.DEFAULT_AGENT_ID` 统一引用，前端新增 `DEFAULT_AGENT_ID` 常量。
- 前端品牌文案从"真格 App"更新为"Nexus"。
- `persistence` 包重命名为 `repository`（`agent/service/persistence` → `agent/service/repository`，`agent/api/persistence` → `agent/api/repository`）。
- 前端配置文件合并 `options.ts` + `runtime-config.ts` 为统一的 `options.ts`。

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

### Added
- 新增 main agent 编排 CLI（`agent/cli.py`），支持 `list_agents`、`create_agent`、`validate_agent_name`、`list_rooms`、`create_room`、`add_room_member` 命令。
- 新增 main agent 编排服务层，对外暴露多 Agent 与 Room 管理能力。
- 新增 `nexus-manager` skill，规范化 YAML frontmatter、命令参考、Workspace 规则与操作流程文档。
- 首页 App 对话交互完整打通：双态过渡、消息收发、权限决策、中断/重试、创建协作直达首条对话。
- 支持 `AskUserQuestion` 自定义回答选项。
- 新增 `room conversation` CRUD API，并接通 room 页面真实的新建/删除对话、重命名 room、增删成员与删除 room 管理能力。

### Fixed
- 修复首页主对话长内容溢出与自动贴底滚动问题。
- 修复 `AskUserQuestion` 回答后被中断的问题。

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
- 新增 `docs/nexus-core-technical-doc.md`，补充项目技术架构、核心链路与技术交流提纲。

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
