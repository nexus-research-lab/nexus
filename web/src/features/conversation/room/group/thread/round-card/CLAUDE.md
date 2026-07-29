# Room Round Card

## 职责

- `group-round-card-model.ts` 聚合一轮内的用户消息、Agent 执行身份、人工介入请求和权威展示顺序。
- `group-round-card-group.tsx` 按统一 entries 顺序编排用户消息与 Agent slot，不按运行状态重排。
- `group-agent-reply.tsx` 只把 entry 装配进 `group-agent-execution-shell.tsx`；后者从 pending、streaming、waiting 到 terminal 始终复用同一个 `MessageItem` Assistant 外壳，并只在无消息的新 handoff 首次出现时做一次合成层淡入。`group-agent-execution-model.ts` 从 pending 起复用“正在思考”，正文流式时复用“正在回复”，其余按已有 slot/message/permission 证据翻译共享 activity 语义，并为没有任何 Assistant 消息的失败/停止终态补齐窄投影，不复制运行状态机、正文或 result 规则。
- 进行中的 Agent 卡片复用 assistant 消息通道的宽度与响应式基线，禁止在 feed 通道内再次居中或叠加横向缩进。
- `thread-action-button.tsx` 是主 Feed 中 Thread 开关的唯一视觉实现。
- 相邻 Agent 依靠身份头、留白与头像同轴的短提示建立边界，不使用贯穿正文列的横线；Markdown `hr` 只表达模型正文语义。

## 边界

- 身份映射由群聊面板完整提供，本目录不接受缺失目录后再补空对象。
- 活动状态与最终正文沿用共享 `MessageItem` 投影；Room 只从同一 execution entry 投影“思考/执行/回复/等待”语义，并补充执行身份、Thread/停止动作和无消息终态。
- 公区保留进行中 slot 的身份、加载标识和 Thread 入口；流式正文直接在同一 Assistant 内容列增长，不得先压成单行摘要再整体切换终态组件。
- 卡片与 Thread 选择态以 `agent_round_id` 隔离；同 Agent 的历史执行与当前执行不得共用 React key 或展开态。
- 单目标 guide 优先按持久化的消费方 `agent_round_id` 归卡；只有旧历史缺少该身份时才按时间兼容。
- Agent 卡片只按稳定 `display_order` / slot 顺序排列；pending、streaming、terminal 状态变化以及 guide 到达都不得移动已展示卡片。
- 公区只投影公开回复正文与 Composer-owned 人工介入的只读等待证据，不展开内部工具过程；同一执行外壳内的内容增长不得依赖滚动层补救组件整体替换。
- 公区终态过滤 thinking；无公开正文的 `done` 槽位仍保留同一完成态 `MessageItem` 外壳、终态说明与 Thread 入口，不得退回活动状态卡。
- 每个 pending interaction 只由 Composer 提供操作面；Agent 槽位与终态正文不得重复渲染结构化问题或批准按钮。
- 带 root round 与 `agent_round_id` 的权限若对应 execution 已由 lifecycle 收口，必须在卡片与 root fallback 两层同时过滤，不能以通用交互卡重新出现。
- 所有会让 runtime 等待用户响应的请求都由 Composer 原位替换输入框并成为唯一操作面；Room 主卡片与 Thread 只保留请求身份、等待状态和执行结果等只读证据，不得重复批准、回答或计划确认按钮。
- 活动卡片的停止按钮只绑定自身 `stopAgentRoundId`，调用链必须传递 `agent_round_id`，终态 result 到达后不得被迟到的 streaming 事件复活。
