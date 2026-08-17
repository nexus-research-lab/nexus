# Task Dialog Resources

- 所有依赖请求共用 `use-dialog-resource.ts` 的请求号与过期响应拒绝逻辑。
- `task-dialog-resource-model.ts` 统一生成请求键、选项和当前会话资源投影；Hook 只执行请求并组合结果。
- 历史脚本任务不加载会话；Agent 任务仅在执行或投递确实需要时加载会话。Agent 候选只接收结构化 DM/active-paired IM Session（Room-backed DM 即使带 `room_id` 仍合法）；Room 候选只接收真实 group Room，把成员 Session 折叠为共享 conversation，再从该 conversation 的实际 Session 索引生成成员 Agent 候选。
- 资源层返回具体选项与状态，不持有表单选择。
- 表单只消费资源的 `loading/error`，原始资源项不得越过资源层。
