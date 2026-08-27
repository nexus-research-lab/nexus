# Conversation Todos

对话任务条的纯投影域。

## 职责

- `todo-status-model.ts` 将 SDK/system 状态与 task progress 文本归一为 `TodoItem.status`。
- `runtime-task-model.ts` 将不同消息源投影为统一任务候选，再通过单一入口归并到任务 Map。
- `task-tool-names.ts` 统一声明对话任务工具名，供任务条与消息隐藏规则共同消费。
- `task-list-tool-model.ts` 将 `TaskCreate` / `TaskList` / `TaskUpdate` 的结构化结果投影为会话级任务列表。
- `todo-projection-model.ts` 单次扫描消息建立轮次索引，选择最新任务轮次后按显式展示策略投影任务条；Room legacy 面板按 `agent_id` 隔离多份进程，WorkGraph 节点局部进程另按 `agent_id + agent_round_id` 保留每次精确 Task run。
- `use-conversation-todos.ts` 只负责单进程、Room 多 Agent 进程与精确 Agent round Task run 投影的 React memo 和结果引用稳定。

## 不变量

- 只处理与当前 session 等价的消息；旧 Todo/runtime 事件按轮次投影，新 Task List 按 session 持续投影。
- 新的可见对话轮次开始后，上一轮 TodoWrite 计划必须立即退出任务条，不等待新轮次再次写计划。
- Task List 只消费当前最新 runtime session，避免 Room 多 Agent 或 runtime 重建后串入旧任务文件。
- Room 每个 Agent 独立选择自己的最新 runtime session；进程最近事件位置只用于默认来源选择，不得改变成员目录顺序。
- WorkGraph 只能消费同时具备 Agent 与 Agent round 身份的 Task run；缺失关联键的本地任务继续留在 legacy 进程，不得猜挂到节点。
- 一旦观察到 Task List 工具，以它的列表快照和增量更新为真相，不再回退到旧 TodoWrite 计划。
- Task List 优先消费结构化结果；文本只作为旧历史与非标准 runtime 的稳定降级路径。
- TodoWrite 在消费边界校验每个任务，并把当前 `content` 与历史 transcript 的 `task`、`activeForm` 统一投影为 `TodoItem`；未知或残缺任务不得进入 UI。
- 同一轮 TodoWrite 计划与 runtime task 始终合并，不根据消息 role 改变规则。
- 消息源只解析身份、内容候选和状态；旧内容、旧表单与 Map 写入规则不得重复实现。
- 状态事件只有显式结构化描述可以改写任务标题；`task_notification` 只收口状态并保留已有任务身份，孤立通知最多使用短 `summary`，不得把结果正文投影进任务条。
- 状态别名用数据表维护，不在视图或轮次扫描中复制分支。
- 无计划 runtime、有效计划和隐藏计划使用统一策略表；轮次选择不参与展示规则判断。
