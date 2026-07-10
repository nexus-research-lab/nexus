# Task Dialog Resources

- 所有依赖请求共用 `use-dialog-resource.ts` 的请求号与过期响应拒绝逻辑。
- 脚本任务不加载会话；Agent 任务仅在执行或回复确实需要时加载会话。
- 资源层返回具体选项与状态，不持有表单选择。
