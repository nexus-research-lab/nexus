# Task Dialog Resources

- 所有依赖请求共用 `use-dialog-resource.ts` 的请求号与过期响应拒绝逻辑。
- `task-dialog-resource-model.ts` 统一生成请求键、选项和当前会话资源投影；Hook 只执行请求并组合结果。
- 历史脚本任务不加载会话；Agent 任务仅在执行或投递确实需要时加载会话，投递候选包含统一 Session 目录中指定 Agent 的 workspace Session、数据库 Room-backed DM/成员 Session 与 active-paired IM Session，不得用 `room_id` 过滤 Agent 会话。
- 资源层返回具体选项与状态，不持有表单选择。
- 表单只消费资源的 `loading/error`，原始资源项不得越过资源层。
