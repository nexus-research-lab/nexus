# Room Goal

当前 Goal 属于 shared Room 时额外读取本文件。

## Lead 身份

Room Goal 属于整个 Room。创建者或当前 assigned lead 负责推进、协调、验收、retarget 和最终状态更新，但 Lead 也是实际执行者，可以规划、研究、整合、自审、接管和交付。

只有 Lead 可以通过模型工具 retarget 或更新 Room Goal 状态；创建 Goal 的当前 Lead 以及后续明确分配的新 Lead，都会在自己的新物理 round 启动时取得当前 exact objective revision，完成/阻塞不要求用户再补发一条“继续”。该权限只属于 Goal MCP，不授予 WorkGraph mutation。其他成员应把证据、缺口或修改建议交给 Lead，公区 `@`/directed-message handoff 只回传证据与控制权，不继承 Goal mutation authority。

## 协作与分工

创建 Goal 后，根据独立交付、成员能力、依赖和审核价值决定是否拆 WorkGraph；不要只因为 Room 里有人就机械分派。需要持久责任时使用 Work Item + Assignment，实质上下文和结果通过 Room 消息或产物传递，Submission/Review 由受管链路回交。`@` 只唤起对话或一次性帮助，不建立责任。

分派后不要重复生产同一交付物。Lead 应聚焦自己的责任、协调、解阻、整合和验证；只有任务原子、其他成员没有净收益或 Lead 自己就是明确 owner 时才整体直接执行。

## 可见协作完成条件

当运行时明确要求当前 Room Goal 具备非 Lead 的可见协作证据时，Lead 必须先让至少一个成员产生与当前 objective revision 相关的实质贡献，再尝试完成。普通 `@` 计划、候选人描述或隐藏协作不构成证据。

首次公开分派应写清具体交付物，并等待成员的可见结果；不要在同一轮立即完成 Goal。涉及隐私、密钥或用户明确要求私下协作时才使用 directed message。结果返回后，由 Lead 基于可见证据继续执行、审核或完成 Objective Alignment。
