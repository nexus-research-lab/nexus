# Room Surface Header

- `room-header-tabs.ts` 独占 DM/Group 共用的 Tab 类型、视图定义与 Tour 锚点。
- 子智能体、工作区和简介始终保留在桌面 Header；常规宽度显示图标与标签，窄容器只隐藏可见标签，不得把视图移进辅助菜单。
- Group Room 成员入口与三个视图使用同一渐进收缩规则：常规宽度显示头像与标签，窄容器显示成员图标，只有进入不超过 559px 的专注模式后才与三个视图一起进入三点菜单。
- Header 通过 `navigationTrailing` 装配 `history/room-history-menu.tsx`；历史不参与右侧辅助面板切换。
- 会话标签之后的视图、历史与成员分别归共享工具区和协作区排布；桌面 Header 不保留三点菜单或“查看引导”入口，DM 与 Group 不得在业务层重复添加组间距或另设控件高度。
- DM 与 Group Header 只装配各自身份、会话标签和领域动作，不复制共享导航结构；活动视图的再次点击由上层统一投影为关闭。
