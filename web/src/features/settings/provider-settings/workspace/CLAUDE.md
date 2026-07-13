# Provider Workspace

- `provider-workspace-model.ts` 定义列表刷新、选择、创建和草稿更新的纯状态迁移。
- `use-provider-workspace.ts` 只负责加载资源、隔离过期请求并向控制器提供 Workspace 命令。
- 刷新结果必须同时匹配当前请求代次；过期请求不得写状态、反馈或全局 Provider 可用性缓存。
- 选择迁移接收已解析的 Provider 记录，避免纯模型重复查找或依赖 React 闭包。
