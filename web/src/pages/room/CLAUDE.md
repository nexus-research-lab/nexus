# Room 页面

- `room-page.tsx` 只装配控制器的职责分组、浏览器协调器与视图，不持有服务端资源规则。
- 右栏的鼠标开始与键盘宽度请求沿 Surface 交给同一 workspace 尺寸 owner；页面不解释像素或百分比边界。
- `controller/` 负责 Room 数据、命令和派生模型；异步结果必须绑定当前 `roomId`。
- `orchestration/` 负责 URL、导航、页面级事件和 Tour，不得下沉到领域 Feature 或通用 Hook 目录。
- `room_deleted` 是服务端已确认事实，当前页面直接离开失效路由，不以旧页面快照二次推断。
- 无完整 Room 上下文时的 GroupRouteEntry 保留降级导航与精确 Room 最近会话过滤；三个入口直接组合 WorkspaceCatalogCard 的主动作和共享排版，不再依赖单消费者工作区动作封装。

页面加载提示从对应双语目录读取，并交给 WorkspaceLoadingState 统一呈现。
