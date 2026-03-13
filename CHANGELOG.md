# Changelog

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
