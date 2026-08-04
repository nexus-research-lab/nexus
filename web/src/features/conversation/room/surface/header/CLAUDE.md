# Room Surface Header

- `room-header-tabs.ts` 独占 DM/Group 共用的 Tab 类型、视图定义与 Tour 锚点。
- `use-room-header-overflow-tabs.ts` 与共享 Header 样式共同定义桌面渐进收纳：按简介、工作区、子智能体、工作图的顺序把空间不足的视图移进辅助菜单，断点必须同步修改，禁止退化成独立“面板”下拉。工作图在可用空间内最后收纳，使正在执行的拓扑始终容易抵达。
- `room-header-guide-menu.tsx` 独占桌面辅助菜单的锚点和开关状态；Group Room 的成员入口在成员头像被隐藏后仍保留在该菜单中。
- Header 通过 `navigationTrailing` 装配 `history/room-history-menu.tsx`；历史不参与右侧辅助面板切换。
- 会话标签之后的视图、历史、成员与辅助菜单分别归共享工具区和协作区排布，DM 与 Group 不得在业务层重复添加组间距或另设控件高度。
- DM 与 Group Header 只装配各自身份、会话标签和领域动作，不复制共享导航结构。
