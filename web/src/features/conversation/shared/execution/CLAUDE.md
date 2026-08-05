# Execution WorkGraph

- `use-execution-resource.ts` 只读取后端 `ExecutionView`，以 WebSocket activity invalidation 合并刷新；30 秒活动态检查只用于断线恢复，并在读取失败而保留旧快照时显式暴露 stale/last-successful-at，不派生第二套状态机。
- `execution-process-panel.tsx` 是 Composer 上方的实时 Agent Dock：只显示去重后的一级 Agent 责任节点，以轻量连线保留协作链路感知，工作图入口固定在右侧；绿色只表示该 Agent 正在运行。点击带精确 `agent_round_id` 的头像跳转到对应消息轮次，图标按钮打开右侧完整工作图。这里不得复制 DAG，也不得混入 Tool、Gate 或 Subagent。
- `execution-workgraph-surface.tsx` 是 Header “工作图”入口对应的完整辅助 Surface；它与缩略入口消费 `room-surface-shell.tsx` 中同一个 `ExecutionResource` 和同一组 Task runs，不得自行轮询、复制状态机或制造第二份图快照。
- `execution-node-avatar.tsx` 是 Agent Dock 与完整节点图共用的 Agent/Subagent/未分配节点原语；完整图投影生命周期，Dock 投影实时活动，二者不得复用绿色表达不同状态。Subagent 使用 child runtime identity 的稳定头像，不借用父 Work Item owner 头像。
- `execution-process-model.ts` 只做标签、当前节点、一级 Agent 去重、Lead/coordination 优先顺序、稳定头像 identity、运行图可见性与 dependency 深度投影。无 Plan 的 runtime-only Agent Loop 也能暴露同一 WorkGraph UI，但不得因此冒充 managed Execution、Assignment 或 Goal。`coordination` 只表达 Lead 对已声明根工作项的责任，禁止冒充已经发生的 dispatch；`parent_work_item_id` 是 containment，禁止进入 readiness 或布局边。
- `execution-node-task-model.ts` 只允许以 WorkAttempt 的 `executor_agent_id + agent_round_id` 精确关联 Conversation Task run；缺失或不匹配时不展示，禁止按 Agent、消息位置或时间猜测节点归属。
- `execution-node-task-list.tsx` 只把命中的 runtime-local Task 作为节点局部步骤展示，不把 Task 状态提升为 Work Item、Submission 或 Acceptance。
- `execution-workgraph-layout.ts` 保持主责任图横向依赖/dispatch 层级，把同一 Agent 的可见 Tool/Subagent 按结构化父身份向下组织成紧凑子图；后端显式边优先，历史快照仅在可见节点已有 `parent_node_id`、同一 `agent_round_id` 或唯一同责任 Agent 时补出 invoke/spawn 方向边，禁止留下孤点或从文字猜关系。`loop_back` 只表达失败或 Gate 后控制权已经返回，`retry` 只表达 Agent 已明确选择并精确关联的再次执行；两者都是控制边，不参与父级或深度推断。托管 WorkGraph 只接收已绑定 Work/Review/Coordination lane 的 runtime Agent，普通 Room reply root 留在 Conversation activity。
- `execution-workgraph-interaction-model.ts` 与 `execution-workgraph-controls.tsx` 只维护当前用户的折叠、全文搜索、缩放、适配和当前节点定位；搜索会展开命中节点的祖先，折叠只向 layout 传递隐藏集合，禁止修改权威拓扑、运行状态或 Agent 路线。
- `execution-workgraph-canvas.tsx` 观察自身容器并负责低对比度工作板网格、无文字子图轮廓、整条线鼠标命中区与边中点原生键盘按钮，以及贴近所选节点或边、自动避让左右边界的悬浮检查器。边检查器只解释来源、目标、exact Run identity、观测时间与已发生语义，不提供路线 mutation。`execution-node-run-history.tsx` 在节点检查器内展示每次 NodeRun 的结果/错误、耗时与 exact structured Artifact，并只把安全的 workspace 相对路径交给既有 Workspace 打开链路。历史 child run 的 `detail` 节点默认不进入主图，但失败/进行中、参与控制边、带 Artifact 或携带通用 `workgraph_visibility` 提示的 Tool 按结构事实进入子图；禁止按具体工具名决定能力或可见性，其他结果事实仍保留在父节点详情中。
- 后端 runtime projection 超出独立窗口时必须返回 `runtime_*_total` 与 `runtime_*_truncated`；Surface 显式显示 partial，读取失败但仍保留最后成功快照时显式显示 stale，禁止把截断、断线或旧数据伪装成完整实时图。
- 节点检查器以结构化 owner 为责任人真相源；自由文本 objective 若以同一 owner 身份开头，只在展示层消除这段冗余前缀，不改写后端 Work Item。
- Composer 上方入口不承载交付契约工作台；完整领域数据仍由后端 read model 保留，节点进度只回答当前任务、走到哪里、由谁负责。
- UI 不提供状态写入按钮；编排 mutation 仍由模型语义工具和后端 authority fence 负责，详情中的失败与回连不得被解释成服务端自动重试或固定路由。
