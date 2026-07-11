# Room 桌面布局

- 入口只选择 DM 或 Room Thread 上下文；`RoomSurfaceContent` 组合主聊天和右栏。
- 控制器负责 Tab、联系人请求、子智能体来源和宽侧栏联动，视图不重复派生。
- Header、辅助面板和 Thread 装配各自消费窄接口；消息轨道统一复用 `conversation/shared/thread/`，聊天面板必须常驻挂载。
- 历史、工作区和简介面板保持挂载，通过数据表统一控制可见性。
- Room 写命令使用页面控制器已绑定的作用域，Header 不重复传递 `roomId`。
