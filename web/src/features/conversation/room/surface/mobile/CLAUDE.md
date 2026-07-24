# Room Focus Surface

- `room-mobile-surface.tsx` 在窄窗单会话模式下维护 Switcher/Overlay 状态并装配共享聊天表面；不得重新引入桌面侧栏或压缩桌面 Header。
- `room-mobile-header.tsx` 只负责返回聊天目录、当前会话入口及其展开状态，以及历史/更多操作的尾部插槽。
- `room-mobile-actions-menu.tsx` 是窄窗新建会话、群聊成员、子智能体、工作区、简介和引导操作的统一入口；成员仅在 Group Room 中出现，并复用桌面成员管理事务。
- `room-mobile-auxiliary-overlay.tsx` 独占窄窗工作区与简介全屏层，关闭后必须回到原会话上下文。
- `room-mobile-conversation-switcher.tsx` 独占会话列表展示与选择交互；它从触发它的顶栏向下展开，并复用一级目录的轻边框、图标框、活动标记和行高。Switcher 本体必须把 `surface-panel` 轻量混入当前暖色环境底面并保持半透明模糊，标题区与列表区共享同一材质，禁止混入不透明的 Paper 或 Popover 材质；当前会话使用中性活动底面，品牌色仅留在窄标记与当前状态提示。
- Thread 与子智能体全屏层分别由各自 Overlay 组件装配，不回流到主表面。
- 窄窗 Thread 与桌面右栏共用 `group/thread/live/` 的面板模型，不自行补全身份或动作。
- DM/Group 聊天参数统一经过 `../room-chat-surface.tsx`；专注模式不得复制 Panel 分支。
