# Room Focus Surface

- `room-mobile-surface.tsx` 只在不超过 559px 的真正窄窗中维护 Switcher/Overlay 状态并装配共享聊天表面；中等宽度继续使用渐进压缩的桌面 Header，不得过早切换单会话模式。
- `room-mobile-header.tsx` 只负责返回聊天目录、当前会话入口及其展开状态，以及历史/更多操作的尾部插槽；桌面窄窗下复用统一的拖动区域与原生窗口按钮避让契约，并与桌面 Header 共用不改变 viewport 几何的非交互下缘渐隐。
- `room-mobile-actions-menu.tsx` 只在最小专注模式中组装新建会话、群聊成员、工作图、子智能体、工作区和简介，顺序与可用态由 `room-mobile-actions-model.tsx` 统一投影；工作图入口始终存在，不以当前是否已有托管图作为开启条件。不提供“查看引导”，成员仅在 Group Room 中出现并复用桌面成员管理事务。
- `room-mobile-auxiliary-overlay.tsx` 独占窄窗工作图、工作区与简介全屏层，关闭后必须回到原会话上下文；工作图复用桌面同一 `ExecutionWorkGraphSurface`，资源为空时保持 Overlay 并显示统一空态，不得主动关闭。
- `room-mobile-conversation-switcher.tsx` 独占历史会话列表展示与选择交互，并复用 `history/room-history-model.ts` 的内部草稿过滤；过滤结果只供展示和计数，不得回写标签目录、当前选择或路由。外部 Session 保持既有展示规则，首条用户输入使内部会话退出草稿态后才进入列表。
- Switcher 从触发它的顶栏向下展开，并复用一级目录的轻边框、图标框、活动标记和行高。Switcher 本体必须把 `surface-panel` 轻量混入当前暖色环境底面并保持半透明模糊，标题区与列表区共享同一材质，禁止混入不透明的 Paper 或 Popover 材质；当前会话使用中性活动底面，品牌色仅留在窄标记与当前状态提示。
- Thread 与子智能体全屏层分别由各自 Overlay 组件装配，不回流到主表面；Room 子智能体全屏层与桌面右栏共用调用 Agent 选择和过滤规则。
- 窄窗 Thread 与桌面右栏共用 `group/thread/live/` 的面板模型，不自行补全身份或动作。
- DM/Group 聊天参数统一经过 `../room-chat-surface.tsx`；专注模式不得复制 Panel 分支。
