# Changelog

## 2026-03-10

### Fixed
- 修复后端 `session_manager` 导入缺失问题。
- 修复 HTML 嵌套 button 验证错误，改为语义化结构。
- 修复成本账本始终为 0 的问题：补齐 SDK `ResultMessage` 的 dataclass 解析，修正消息保存链路中的 `agent_id` 归一化，并支持从历史 `messages.jsonl` 自动回填 Session / Agent 成本汇总。
- 修复左侧 session 区域 `msgs` 长期显示为 0 的问题：前端在消息加载和实时更新后会同步本地 session 快照，及时刷新 `message_count` 和最近活动时间。
- 修复 workspace 文件编辑面板展开/收起的跳动：改为常驻挂载 + 显式宽度过渡，减少 flex 重排导致的动画突变。
- 进一步调整 workspace 文件编辑布局：将 editor 从 workspace 侧栏中拆成独立中间面板，避免展开时推挤顶部工具条和侧栏。
- 修复 Agent Space 左右侧栏宽度不一致的问题，统一 `Workspace` 与 `Agent State` 的固定宽度。
- 修复文件编辑器关闭后仍保留中间 gap 的问题，恢复 `Workspace`、`Session`、`Agent State` 三栏等距。

### Added
- 新增自定义对话框组件 (ConfirmDialog, PromptDialog)，替代 window.confirm/prompt。
- 添加暗黑模式支持 (prefers-color-scheme: dark)。
- 添加全局设计令牌 (success, warning, scrollbar 变量)。
- 添加统一滚动条样式 (Webkit + Firefox)。
- 添加全局 kbd 元素样式。
- 新增基于 workspace 文件存储的成本账本能力，按 Session 持久化 `telemetry_cost.jsonl` 与 `telemetry_cost_summary.json`。
- 新增 Agent 级成本汇总文件 `telemetry_cost_summary.json`，支持按 Agent 聚合累计 token / cost。
- 新增 Session / Agent 成本汇总 API，供前端右侧状态栏读取权威成本数据。
- 新增权限运行时辅助层，支持序列化 Claude SDK `permission_suggestions`、构建权限风险摘要，并将用户决策回写为 `updated_permissions`。

### Accessibility
- 为所有交互按钮添加 aria-label 属性。
- 添加 focus-visible 聚焦样式。
- 为进度条添加 ARIA progressbar 属性。
- 使用语义化颜色变量 (text-destructive, text-success, text-warning)。

### Changed
- 新增项目级 `SYSTEM_PROMPT.md` 基础提示词层，并支持通过 `BASE_SYSTEM_PROMPT` / `BASE_SYSTEM_PROMPT_FILE` 在 workspace 规则之前强制注入身份设定。
- Web 首页重构为 B 端控制台骨架，采用 `Agent Directory -> Agent Space` 的两层信息架构。
- 在 Agent Space 内新增顶部快速切换器，支持不返回目录页直接切换 Agent，兼顾结构分层与多线程操作效率。
- 新增 Session Rail 与 Agent Inspector，为后续承接权限队列、运行审计、Workspace/Memory 面板预留结构位置。
- 全局设计 token 从赛博终端风格收口为更稳定的控制台视觉语言，统一页面底色、字体和 panel 表达。
- 首页文案进一步收口为简洁后台表达，移除解释型和营销式描述。
- 首页主色调整为更克制的深灰蓝方案，降低装饰性和视觉噪声。
- Agent Space 布局进一步压缩，减少头部与侧栏占位，把更多空间留给会话主区和实际可操作面板。
- 新增 workspace 文件浏览、读取和写回能力，右侧面板可直接操作本地 workspace 文件，并在保存后刷新 Agent 活跃 session。
- 移除首页和 Agent Space 的大标题区，把目录页头部压缩成小工具栏，减少重复信息占位。
- 继续放大 workspace 编辑区，收紧非核心面板，让 Session Space 与 Workspace 成为真正的双主区。
- Agent Space 进一步按 harness capability 重构为三栏布局：左侧 `Virtual filesystem + Context`，中间 `Session Space`，右侧 `Agent State`。
- Workspace 文件编辑器改为按需从左侧滑出，展开后与对话主区对半分配，降低常驻编辑器对会话空间的挤压。
- 右侧状态栏改为 runtime、context capacity、token/cost、planning、orchestration、policy 六个状态区，显式映射后续 harness telemetry 能力。
- workspace 左栏升级为目录树视图，并补齐文件/目录创建、删除、重命名操作，保持与本地 workspace 目录同步。
- Agent Space 进一步压缩重复信息：移除重复标题文本，文件树改为目录递归结构，点击同一文件可收起编辑器，并支持拖拽调整 editor / session 分栏宽度。
- 会话区 todo/plan 状态迁移到右侧 `Agent State`，中间 header 收口为更轻的 session telemetry 条。
- 右侧 `Agent State` 接入真实 telemetry：基于 result 消息展示真实 token/cost，基于 pendingPermission 展示审批队列，基于 toolCalls 展示 trace timeline。
- 右侧 `Agent State` 收口为只展示成本能力：移除 approval / trace telemetry，改为读取后端成本账本汇总，展示 Session / Agent 成本、token、缓存命中和最近一次执行耗时。
- Agent 内部运行文件统一下沉到 workspace 隐藏目录 `.agent/`，包括 `agent.json`、Session 元数据/消息日志和成本账本，避免污染用户可见 workspace 根目录。
- 权限链路对齐 Claude Agent SDK：后端现在会把 `permission_suggestions` 传到前端，并支持把“仅本次 / 写入 session / 项目 / 本地 / 用户设置”的授权决策回传给 SDK。
- WebSocket 权限策略补齐关闭与超时兜底：连接关闭后会立即唤醒挂起中的权限请求并拒绝，避免继续向已关闭连接发送审批事件。
- Agent 配置中的 `setting_sources` 重新透传到 SDK，不再被误过滤；前端也新增 `local` 来源，并将其文案收口为同时作用于技能和权限设置加载。
- Agent 配置页补充 `bypassPermissions` 风险提示，明确 `allowed_tools` 不会约束全放行模式，避免误配造成错误安全感。

## 2026-03-09

### Changed
- 收口 Session 边界：Session API 不再承载执行配置，执行参数统一归 Agent 管理。
- 默认 Agent 路由改为配置化，通过 `DEFAULT_AGENT_ID` 显式控制默认路由策略。
- Agent 更新配置或 workspace 后，会主动失效内存中的活跃 SDK session，后续消息按最新 Agent 配置懒加载重建。

### Docs
- 在仓库规则中补充“一个完整需求对应一个提交”和“必要时同步更新 changelog”的协作约定。
