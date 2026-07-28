# Room Round Card

## 职责

- `group-round-card-model.ts` 聚合一轮内的用户消息、Agent 执行身份、人工介入请求和权威展示顺序。
- `group-round-card-group.tsx` 按统一 entries 顺序编排用户消息与 Agent slot，不按运行状态重排。
- `group-agent-reply.tsx` 保持同一 slot 组件身份：运行中使用 `group-agent-status-card.tsx` 摘要，进入终态后在原位展示完整公开结果。
- 进行中的 Agent 卡片复用 assistant 消息通道的宽度与响应式基线，禁止在 feed 通道内再次居中或叠加横向缩进。
- `thread-action-button.tsx` 是主 Feed 中 Thread 开关的唯一视觉实现。

## 边界

- 身份映射由群聊面板完整提供，本目录不接受缺失目录后再补空对象。
- 状态摘要来源、色调和 Markdown 展示由完整规则表决定，视图不复制状态判断。
- 公区保留进行中 slot 的身份、加载标识和 Thread 入口，但不显示准备、思考、回复等空占位文案。
- 卡片与 Thread 选择态以 `agent_round_id` 隔离；同 Agent 的历史执行与当前执行不得共用 React key 或展开态。
- 单目标 guide 优先按持久化的消费方 `agent_round_id` 归卡；只有旧历史缺少该身份时才按时间兼容。
- Agent 卡片只按稳定 `display_order` / slot 顺序排列；pending、streaming、terminal 状态变化以及 guide 到达都不得移动已展示卡片。
- 运行态摘要不得把工具过程扩成公区正文；终态公开结果在同一槽位直接显示，滚动层负责在大块切换时保住用户正在阅读的位置。
- 无公开正文的 `done` 槽位仍切换到完成态 MessageItem 外壳并显示终态说明，不得退回活动状态卡。
- 每个 pending interaction 只能在同一 Agent 槽位投影一次；终态正文与独立确认模块不得重复渲染同一个结构化问题。
- 所有会让 runtime 等待用户响应的请求都必须在 Room 主卡片展示完整内容并可直接处理；工具审批、结构化回答、计划确认及后续新增类型均适用，Thread 只同步详情，不得成为解除执行阻塞的唯一入口。
- 活动卡片的停止按钮只绑定自身 `stopAgentRoundId`，调用链必须传递 `agent_round_id`，终态 result 到达后不得被迟到的 streaming 事件复活。
