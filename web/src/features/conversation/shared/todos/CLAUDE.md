# Conversation Todos

对话任务条的纯投影域。

## 职责

- `todo-status-model.ts` 将 SDK/system 状态与 task progress 文本归一为 `TodoItem.status`。
- `runtime-task-model.ts` 将 system task 事件和 assistant `task_progress` 块归并到同一任务 Map。
- `todo-projection-model.ts` 单次扫描消息建立轮次索引，再投影当前任务条。
- `use-conversation-todos.ts` 只负责 React memo 与结果引用稳定。

## 不变量

- 只处理与当前 session 等价的消息，无 `round_id` 的任务事件不参与轮次投影。
- 同一轮 TodoWrite 计划与 runtime task 始终合并，不根据消息 role 改变规则。
- 状态别名用数据表维护，不在视图或轮次扫描中复制分支。
