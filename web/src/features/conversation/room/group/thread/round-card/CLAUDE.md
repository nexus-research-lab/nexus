# Room Round Card

## 职责

- `group-round-card-model.ts` 聚合一轮内的用户消息、Agent 身份、权限和状态摘要。
- `group-round-card-group.tsx` 编排用户消息、完成回复和进行中状态卡片。
- `group-completed-reply.tsx` 与 `group-agent-status-card.tsx` 只渲染各自状态，不重新筛选轮次数据。
- `thread-action-button.tsx` 是主 Feed 中 Thread 开关的唯一视觉实现。

## 边界

- 身份映射由群聊面板完整提供，本目录不接受缺失目录后再补空对象。
- 状态摘要来源、色调和 Markdown 展示由完整规则表决定，视图不复制状态判断。
- 权限操作只处理当前卡片收到的首个待确认请求；问题型请求进入 Thread 作答，缺少请求时也进入 Thread 查看详情。
