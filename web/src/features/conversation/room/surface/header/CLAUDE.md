# Room Surface Header

- `room-header-tabs.ts` 独占 DM/Group 共用的 Tab 类型、视图定义与 Tour 锚点。
- `room-header-guide-menu.tsx` 独占桌面辅助菜单的锚点和开关状态；Group Room 的成员入口在成员头像被隐藏后仍保留在该菜单中。
- Header 通过 `navigationTrailing` 装配 `history/room-history-menu.tsx`；历史不参与右侧辅助面板切换。
- DM 与 Group Header 只装配各自身份、会话标签和领域动作，不复制共享导航结构。
