# Room 页面

- `room-page.tsx` 只装配页面控制器、浏览器协调器与视图，不持有服务端资源规则。
- `controller/` 负责 Room 数据、命令和派生模型；异步结果必须绑定当前 `roomId`。
- `orchestration/` 负责 URL、导航、页面级事件和 Tour，不得下沉到领域 Feature 或通用 Hook 目录。
- `room_deleted` 是服务端已确认事实，当前页面直接离开失效路由，不以旧页面快照二次推断。
