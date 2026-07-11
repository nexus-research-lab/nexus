# Room Group Thread

## 职责

- `group-thread-state.ts` 与 `group-thread-context.tsx` 只维护当前 Thread 目标和开关命令。
- `live/` 独占实时会话切片、纯面板投影与生产消费 Hook。
- `group-round-card-model.ts` 一次完成用户消息、Agent 身份、权限和完成状态投影。
- `group-round-card-group.tsx` 只编排主时间线卡片，完成回复和 Thread 动作使用独立视图。
- `group-agent-status-card.tsx` 只渲染进行中 Agent 状态，不重复筛选轮次数据。

## 边界

- 控制上下文不承载消息、权限或回调，避免实时流更新整棵 Room 子树。
- 实时 Store 属于 Thread 私有实现，不从全局 `store/` 暴露协议。
- 桌面与移动端只消费同一个面板模型，不重复补全 Agent 身份或动作能力。
- Thread 开关按钮和展示文案只保留一个实现，状态卡片不得复制样式或字符串。
