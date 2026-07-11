# Room Mobile Surface

- `room-mobile-surface.tsx` 只维护移动端 Sheet/Overlay 状态并装配共享聊天表面。
- `room-mobile-header.tsx` 只负责返回、会话入口和子智能体入口。
- `room-mobile-conversation-sheet.tsx` 独占会话列表展示与选择交互。
- Thread 与子智能体全屏层分别由各自 Overlay 组件装配，不回流到主表面。
- DM/Group 聊天参数统一经过 `../room-chat-surface.tsx`；移动端不得复制 Panel 分支。
