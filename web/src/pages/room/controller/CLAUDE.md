# Room 页面控制器

- 根控制器只组合 Store、资源、模型、命令和 Workspace 控制器，并按 `room / conversation / agent / actions / workspace` 输出职责分组。
- `model/` 只投影 Room 上下文、Session 身份和会话快照。
- `commands/` 封装写操作与刷新策略，页面不直接调用 Room API。
- 控制器不读取 React Router，也不负责页面跳转。
- 资源 Hook 不暴露未消费的错误或恒定状态；页面状态必须对应真实加载阶段。
- `use-room-page-data.ts` 在进程内按 owner 代次保留每个 Room 最近一次服务端确认的上下文：回访时直接渲染页面骨架并后台刷新，避免切换 Room 整页退回全屏加载；缓存起步的快照在本次读取失败时丢弃。`status.isHydrated` 只决定能否渲染，根路由选择与改写使用 `status.isRouteReady`，必须等本次服务端读取确认，不得按缓存旧目录落到错误会话。
