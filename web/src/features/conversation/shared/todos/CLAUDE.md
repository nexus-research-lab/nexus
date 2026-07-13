# Conversation Todos

对话任务条的纯投影域。

## 职责

- `todo-status-model.ts` 将 SDK/system 状态与 task progress 文本归一为 `TodoItem.status`。
- `runtime-task-model.ts` 将不同消息源投影为统一任务候选，再通过单一入口归并到任务 Map。
- `task-tool-names.ts` 统一声明对话任务工具名，供任务条与消息隐藏规则共同消费。
- `task-list-tool-model.ts` 将 `TaskCreate` / `TaskList` / `TaskUpdate` 的结构化结果投影为会话级任务列表。
- `todo-projection-model.ts` 单次扫描消息建立轮次索引，选择最新任务轮次后按显式展示策略投影任务条。
- `use-conversation-todos.ts` 只负责 React memo 与结果引用稳定。

## 不变量

- 只处理与当前 session 等价的消息；旧 Todo/runtime 事件按轮次投影，新 Task List 按 session 持续投影。
- Task List 只消费当前最新 runtime session，避免 Room 多 Agent 或 runtime 重建后串入旧任务文件。
- 一旦观察到 Task List 工具，以它的列表快照和增量更新为真相，不再回退到旧 TodoWrite 计划。
- Task List 优先消费结构化结果；文本只作为旧历史与非标准 runtime 的稳定降级路径。
- 同一轮 TodoWrite 计划与 runtime task 始终合并，不根据消息 role 改变规则。
- 消息源只解析身份、内容候选和状态；旧内容、旧表单与 Map 写入规则不得重复实现。
- 状态别名用数据表维护，不在视图或轮次扫描中复制分支。
- 无计划 runtime、有效计划和隐藏计划使用统一策略表；轮次选择不参与展示规则判断。
