# Execution WorkGraph

- `use-execution-resource.ts` 只读取后端 `ExecutionView`，活动态轮询是断线恢复，不派生第二套状态机。
- `execution-process-panel.tsx` 是 Composer 上方的实时 Agent Dock：只显示去重后的一级 Agent 责任节点，以轻量连线保留协作链路感知，工作图入口固定在右侧；绿色只表示该 Agent 正在运行。点击带精确 `agent_round_id` 的头像跳转到对应消息轮次，图标按钮打开右侧完整工作图。这里不得复制 DAG，也不得混入 Tool、Gate 或 Subagent。
- `execution-workgraph-surface.tsx` 是 Header “工作图”入口对应的完整辅助 Surface；它与缩略入口消费 `room-surface-shell.tsx` 中同一个 `ExecutionResource` 和同一组 Task runs，不得自行轮询、复制状态机或制造第二份图快照。
- `execution-node-avatar.tsx` 是 Agent Dock 与完整节点图共用的 Agent/Subagent/未分配节点原语；完整图投影生命周期，Dock 投影实时活动，二者不得复用绿色表达不同状态。Subagent 使用 child runtime identity 的稳定头像，不借用父 Work Item owner 头像。
- `execution-process-model.ts` 只做标签、当前节点、一级 Agent 去重、稳定头像 identity、托管工作图可见性与 dependency 深度投影；只有含 Plan 或 Work Item 的 Execution 能暴露 WorkGraph UI，普通 runtime round 观测图不能让入口常驻。`parent_work_item_id` 是 containment，禁止进入 readiness 或布局边。
- `execution-node-task-model.ts` 只允许以 WorkAttempt 的 `executor_agent_id + agent_round_id` 精确关联 Conversation Task run；缺失或不匹配时不展示，禁止按 Agent、消息位置或时间猜测节点归属。
- `execution-node-task-list.tsx` 只把命中的 runtime-local Task 作为节点局部步骤展示，不把 Task 状态提升为 Work Item、Submission 或 Acceptance。
- `execution-workgraph-layout.ts` 保持主责任图横向依赖/dispatch 层级，把同一 Agent 的可见 Tool/Subagent 按结构化父身份向下组织成紧凑子图；后端显式边优先，历史快照仅在可见节点已有 `parent_node_id`、同一 `agent_round_id` 或唯一同责任 Agent 时补出 invoke/spawn 方向边，禁止留下孤点或从文字猜关系。托管 WorkGraph 只接收已绑定 Work/Review/Coordination lane 的 runtime Agent，普通 Room reply root 留在 Conversation activity。`execution-workgraph-canvas.tsx` 观察自身容器并负责低对比度工作板网格、无文字子图轮廓、始终可见的方向边，以及不遮挡连线的独立节点检查器。检查器只呈现目标、交付、验收、阻塞、提交/审核与精确关联的局部 Task。历史 child run 的 `detail` 节点默认不进入主图。
- 节点检查器以结构化 owner 为责任人真相源；自由文本 objective 若以同一 owner 身份开头，只在展示层消除这段冗余前缀，不改写后端 Work Item。
- Composer 上方入口不承载交付契约工作台；完整领域数据仍由后端 read model 保留，节点进度只回答当前任务、走到哪里、由谁负责。
- UI 不提供状态写入按钮；编排 mutation 仍由模型语义工具和后端 authority fence 负责。
