# Room Round Card

## 职责

- `group-round-card-model.ts` 聚合一轮内的用户消息、Agent 执行身份、权限和权威展示顺序。
- `group-round-card-group.tsx` 按统一 entries 顺序编排用户消息、完成回复和进行中 slot 卡片，不拆成两段重排。
- `group-completed-reply.tsx` 与 `group-agent-status-card.tsx` 只渲染各自状态，不重新筛选轮次数据。
- `thread-action-button.tsx` 是主 Feed 中 Thread 开关的唯一视觉实现。

## 边界

- 身份映射由群聊面板完整提供，本目录不接受缺失目录后再补空对象。
- 状态摘要来源、色调和 Markdown 展示由完整规则表决定，视图不复制状态判断。
- 公区保留进行中 slot 的身份、加载标识和 Thread 入口，但不显示准备、思考、回复等空占位文案。
- 卡片与 Thread 选择态以 `agent_round_id` 隔离；同 Agent 的历史执行与当前执行不得共用 React key 或展开态。
- 单目标 guide 优先按持久化的消费方 `agent_round_id` 归卡；只有旧历史缺少该身份时才按时间兼容。
- 权限操作只处理当前卡片收到的首个待确认请求；问题型请求进入 Thread 作答，缺少请求时也进入 Thread 查看详情。
