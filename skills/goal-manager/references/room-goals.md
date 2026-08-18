# Room Goal

当前 Goal 属于 shared Room 时额外读取本文件。

## Lead 身份

Room Goal 属于整个 Room。创建者或当前 assigned lead 负责推进、协调、验收、retarget 和最终状态更新，但 Lead 也是实际执行者，可以规划、研究、整合、自审、接管和交付。

只有 Lead 可以通过 Goal command retarget 或更新 Room Goal 状态；创建 Goal 的当前 Lead 以及后续明确分配的新 Lead，都会在自己的新 physical round 启动时取得当前 exact objective revision，完成/阻塞不要求用户再补发一条“继续”。该权限只属于当前 Goal command capability，不授予 WorkGraph mutation。其他成员应把证据、缺口或修改建议交给 Lead，公区 `@`/directed-message handoff 只回传证据与控制权，不继承 Goal mutation authority。

## 协作与分工

创建 Goal 后，根据独立交付、成员能力、依赖和审核价值决定是否拆 WorkGraph；不要只因为 Room 里有人就机械分派。需要持久责任时使用 Work Item + Assignment，实质上下文和结果通过 Room 消息或产物传递，Submission/Review 由受管链路回交。`@` 只唤起对话或一次性帮助，不建立责任。

分派后不要重复生产同一交付物。Lead 应聚焦自己的责任、协调、解阻、整合和验证；只有任务原子、其他成员没有净收益或 Lead 自己就是明确 owner 时才整体直接执行。

## 可见协作审计

Room Goal 是否需要协作由当前负责人根据任务事实决定，不由成员数量机械触发。负责人确认 objective 已满足且当前 Room/Execution readiness 通过后，可以直接完成 Goal；非 Lead 协作证据是可选审计事实，不是完成门槛。

若使用 `@` 请求一次性贡献，应写清问题并等待公开实质回复；Goal-attributed 回复会记录为协作证据，但不会创建 WorkBinding。若成员必须承担可追责交付，则使用 WorkGraph/Assignment。任何已启动的 `@` handoff、Assignment、队列或 wake 都应先完成或显式取消，再关闭 Goal。涉及隐私、密钥或用户明确要求私下协作时才使用 directed message；私域回复不成为公开协作证据。
