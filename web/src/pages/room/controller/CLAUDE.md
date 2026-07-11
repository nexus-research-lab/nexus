# Room 页面控制器

- 根控制器只组合 Store、资源、模型、命令和 Workspace 控制器。
- `model/` 只投影 Room 上下文、Session 身份和会话快照。
- `commands/` 封装写操作与刷新策略，页面不直接调用 Room API。
- 控制器不读取 React Router，也不负责页面跳转。
