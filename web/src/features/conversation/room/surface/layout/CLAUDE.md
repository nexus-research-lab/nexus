# Room 桌面布局

- 入口只选择 DM 或 Room Thread 上下文；`RoomSurfaceContent` 组合主聊天和右栏。
- 控制器负责 Tab、联系人请求、子智能体来源和宽侧栏联动，视图不重复派生。简介请求按 owner/Room/当前 Agent 隔离并同步丢弃旧目标；同 Room 的 Session 切换不重置配置栏目，具体联络查询由内容领域更新。
- Header、辅助面板和 Thread 装配各自消费窄接口；消息轨道统一复用 `conversation/shared/thread/`，聊天面板必须常驻挂载。
- `room-surface-split.css` 统一主聊天与 Thread、工作区、子智能体、简介右栏的软分栏：8px 同色隐形拖拽槽配合右栏的轻微明度差，只保留极弱的向左羽化阴影，不使用导航层灰带、硬竖线或中央拖手。
- 桌面 Room/DM Header 保持共享桌面高度基线与拖窗热区；浏览器回退 60px，macOS 跟随原生红灯中心。非交互渐隐由主聊天容器裁剪，禁止越过分栏覆盖工作区，也禁止改成绝对 Header 或用消息 padding 补偿。移动端无并列工作区，渐隐仍从整宽 Header 下缘延伸。
- Thread 右栏只消费 `group/thread/live/` 的面板模型，不直接读取 Chat 会话或实时 Store；右栏只保留共享软分栏边界，内部 Header 与过程轨道不得再叠加装饰分割线。
- `room-side-panel-width.ts` 从同一边界数据生成辅助面板/Thread 的 CSS 限制和实际可调整像素范围；`use-room-side-panel-resize.ts` 观察真正的百分比容器及视口，将键盘请求转换回原百分比 owner。键盘必须从 CSS 限宽后的有效宽度开始，不在不可见的百分比区间逐步移动；容器变化只更新投影，不改写保存偏好。未挂载/零宽测量不启用调整，卸载释放观察与窗口监听。
- 右栏分隔条及面板通过稳定 ID 关联。左右/Home/End、焦点和范围语义由 PanelResizeHandle 统一；叶子面板不复制 min/max CSS 数值或私有键盘处理。
- Header 只接收一个 Room 管理提交命令，不沿布局链传播成员增删和设置更新的底层回调。
- 工作区和简介面板保持挂载，通过数据表统一控制可见性；历史由 Header 的锚定菜单承载，不再打开右侧面板。
- Room 写命令使用页面控制器已绑定的作用域，Header 不重复传递 `roomId`。
