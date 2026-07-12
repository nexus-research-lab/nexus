# 轮次跳转事务

- `round-jump-model.ts`: 定义绑定会话作用域的跳转目标、同一目标判定和轮次加载状态。
- `use-navigation-load-queue.ts`: 串行执行窗口加载，后发目标覆盖排队目标；请求结果必须先通过作用域与代次校验。
- `pending-round-jump-runtime.ts`: 独占逐帧落点的 RAF、重试预算和可见性确认，不发起数据请求。
- `use-pending-round-jump.ts`: 只把当前作用域、时间线和 DOM 引用绑定到落点运行时。
- `use-round-jump.ts`: 只编排跳转意图、目标状态、加载队列和落点控制器。

加载结果使用 `loaded/missing/failed` 判别联合；失效请求不得记录失败、取消新目标或继续排队。
取消目标时必须携带 `navigationRoundId`，由滚动容器协议拒绝清理不同目标。
