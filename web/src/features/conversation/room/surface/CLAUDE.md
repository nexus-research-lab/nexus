# Room Surface

- `room-chat-surface.tsx` 是 DM/Group 与 desktop/mobile 共用的聊天参数装配边界；布局和 Room Host 身份沿唯一上游保持显式。
- `room-chat-error-boundary.tsx` 按会话身份隔离渲染错误，`room-chat-error-view.tsx` 只负责 i18n 回退视图。
- `room-thread-empty-state.tsx` 为桌面和移动端 Thread 检查器提供同一个等待/无额外执行详情状态，禁止两端复制提示样式。 它复用 sm/plain ResourceState 的几何、live status 和 busy 语义；保留 supporting/muted 的行内说明与 16px 装饰 Spinner，不附加空态图标底座、边框或业务动作。
- `header/` 保存 DM/Group 共用导航，`mobile/` 按头部、会话 Sheet 和全屏 Overlay 分离移动端职责。
- Surface Tab 是 Header 导航契约，不得在全局 `types/` 重复定义 UI 状态。

- 根目录保留桌面/移动端入口和可独立展示的业务 Surface。
- `room-conversation-header-edge.css` 只定义桌面与专注模式 Header 自身向正文延伸的非交互下缘羽化，不改变布局高度、拖窗热区或消息滚动几何。
- `room-surface-model.ts` 放置桌面与移动端共享的纯派生，不读取 UI 状态。
- `room-agent-switcher.tsx` 只投影成员与业务触发器，触发状态复用 `UiButton`，菜单生命周期复用 `shared/ui/menu/`；Workspace、Subagent 与 Room 多 Agent 进程切换共享这个入口，不得各自复制按钮状态或头像菜单。Panel 使用固定 112×28px 筛选器并在 12px 头部内边距处对齐，Task 使用最大 144px 的流式宽度；两者共享 16px 头像、左对齐名称、字号与字重节奏。
- Room Agent 简介与联系人详情共用 `features/agents/agent-detail-navigation.ts` 的“身份、技能、记忆、工具、联络”栏目顺序和翻译键；Header 栏目只显示文字，当前项跟随公共 `UiTabs` 的中性底线和 metadata 排版，不恢复私有轻底变体或栏目图标。简介选择公共 compact 密度，不能覆盖单项高度；成员入口与栏目之间只留 8px，栏目横向溢出由公共导航负责。
- `room-subagent-task-surface.tsx` 复用成员切换器，把当前 Session 的全部 Room 子智能体按实际调用者 `host_agent_id` 投影到共享只读任务表面；轮次不得成为隐藏条件。目录重排、名称刷新或增加成员不重置手动调用者；已移除的选择退回有效默认成员并提交该选择，不因成员重新出现而复活。
- 消息中的子智能体任务入口由 Surface 接管导航：桌面打开 `subagents` 右栏，移动端打开同一任务面板，并把精确 `tool_use_id` 与 `host_agent_id` 交给共享 Surface；Room 必须先切换真实调用者再选中任务，Header 手动打开时不得恢复上一次消息定向请求。请求绑定首次收到时的 source，切换会话后消耗，回到旧会话也不重放；手动选调用者会取消仍在等待目录/任务到达的定向请求，新请求才重新取得选择权。
- Room Agent 简介按身份、技能、记忆、工具、联络组织；配置复用 Agent Options 的 Edit 来源工厂，记忆复用 `AgentMemoryView`，不在 Surface 复制用户级 Echo 设置、可编辑字段表或文件式记忆状态。导航选择按 owner/Room/当前 Agent 和显式请求重置；同 Room 的 Session、可见性或目录刷新保留选择。成员移除回到当前 Agent 并消费旧选择，重新出现不复活；重复显式请求可以再次打开同一目标。
- 桌面辅助面板切换时内容起点必须稳定：工作图、简介标签栏、工作区文件栏、子智能体调用者与 Thread 检查器共用 Workspace Surface 的紧凑头部几何与图标基线；移动端工作图复用同一内容 Surface，不另建图资源。
- 桌面辅助面板的 Agent 导航、文件上传和全部缩放拖拽入口必须使用当前界面语言生成可访问名称，不得在业务 Surface 固定中文。
- 桌面分栏、右侧面板与 Thread 编排统一位于 `layout/`。
- Shell 只转交共享右栏宽度和调整命令；像素/CSS 限宽由 layout 投影，键盘与鼠标都更新原百分比 owner，不在 Surface 新建宽度状态。
- 会话历史排序、能力投影、标题编辑和条目视图统一位于 `history/`。
- Thread 的初始等待状态使用共享 `md` muted Spinner；Surface 不自行维护尺寸、颜色、旋转或 reduced-motion class。

- 成员菜单的名称由成员文字给出；装饰头像（含首字母）不参与可访问名称，避免重复播报姓名。
- 成员切换器通过 `lib/agent-selection-options.ts` 显示重名、缺名和不可用的受控选择，通过公共 `UiAgentAvatar` 显示小头像；完整目录只稳定姓名序号，不增加候选。失去候选或受控选择变化同步关闭菜单，名称/顺序刷新不打断使用。缺项不自动显示第一位成员，只有上层业务模型决定回退身份。
