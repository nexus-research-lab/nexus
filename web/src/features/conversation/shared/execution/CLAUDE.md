# Execution WorkGraph

- `use-execution-resource.ts` 只读取后端 `ExecutionView`，活动态轮询是断线恢复，不派生第二套状态机。
- `execution-process-panel.tsx` 是 DM/Room 共用的唯一 WorkGraph 入口；默认沿用 Workspace Task 胶囊，展开后只承载交互 DAG 画布，不把交付契约列表重新塞回主视图。
- `execution-node-avatar.tsx` 是折叠轨迹与展开节点图共用的 Agent/未分配节点原语，只投影责任与状态。
- `execution-process-model.ts` 只做标签、当前节点、可见节点窗口与依赖深度投影，不猜测 Assignment、Attempt、Submission 或 Acceptance。
- `execution-workgraph-layout.ts` 从稳定 Work Item ID 和依赖边纯计算分层坐标；`execution-workgraph-canvas.tsx` 负责头像节点、连线与点击节点后就近浮出的单节点摘要，Plan revision 增删节点时必须原位重排并关闭已经失效的节点浮层。
- Composer 上方入口不承载交付契约工作台；完整领域数据仍由后端 read model 保留，节点进度只回答当前任务、走到哪里、由谁负责。
- UI 不提供状态写入按钮；编排 mutation 仍由模型语义工具和后端 authority fence 负责。
