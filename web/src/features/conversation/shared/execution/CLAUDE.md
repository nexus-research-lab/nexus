# Execution WorkGraph

- `use-execution-resource.ts` 只读取后端 `ExecutionView`，活动态轮询是断线恢复，不派生第二套状态机。
- `execution-process-panel.tsx` 是 Composer 上方的实时缩略入口；折叠态只显示按内容收敛的头像节点轨迹，当前节点标题与进度移入展开面板头部，展开宽度只服从活动 Dock 分配的本地空间，面板主体只承载交互 DAG 画布，不把交付契约列表重新塞回主视图。
- `execution-workgraph-surface.tsx` 是 Header “工作图”入口对应的完整辅助 Surface；它与缩略入口消费 `room-surface-shell.tsx` 中同一个 `ExecutionResource` 和同一组 Task runs，不得自行轮询、复制状态机或制造第二份图快照。
- `execution-node-avatar.tsx` 是折叠轨迹与展开节点图共用的 Agent/Subagent/未分配节点原语，只投影权威节点类型、责任与运行状态。
- `execution-process-model.ts` 只做标签、当前 Agent/Subagent 节点、可见节点窗口与 dependency 深度投影；`parent_work_item_id` 是 containment，禁止进入 readiness 或布局边。
- `execution-node-task-model.ts` 只允许以 WorkAttempt 的 `executor_agent_id + agent_round_id` 精确关联 Conversation Task run；缺失或不匹配时不展示，禁止按 Agent、消息位置或时间猜测节点归属。
- `execution-node-task-list.tsx` 只把命中的 runtime-local Task 作为节点局部步骤展示，不把 Task 状态提升为 Work Item、Submission 或 Acceptance。
- `execution-workgraph-layout.ts` 只从后端 `graph.nodes/edges` 计算 dependency/spawn 分层坐标，旧快照无 graph 时才从真实 dependency 做只读兼容投影；`execution-workgraph-canvas.tsx` 观察自身容器并负责 Agent 头像、Subagent 图标、无文字方向边与点击节点后就近浮出的单节点摘要。历史 child run 的 `detail` 节点默认不进入主图。
- 节点浮层以结构化 owner 为责任人真相源；自由文本 objective 若以同一 owner 身份开头，只在展示层消除这段冗余前缀，不改写后端 Work Item。
- Composer 上方入口不承载交付契约工作台；完整领域数据仍由后端 read model 保留，节点进度只回答当前任务、走到哪里、由谁负责。
- UI 不提供状态写入按钮；编排 mutation 仍由模型语义工具和后端 authority fence 负责。
