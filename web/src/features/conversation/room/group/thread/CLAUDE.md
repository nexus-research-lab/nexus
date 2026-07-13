# Room Group Thread

## 职责

- `group-thread-state.ts` 与 `group-thread-context.tsx` 只维护当前 Thread 目标和开关命令。
- `live/` 独占实时会话切片、纯面板投影与生产消费 Hook。
- `round-card/` 独占主 Feed 的轮次卡片投影与视图。

## 边界

- 控制上下文不承载消息、权限或回调，避免实时流更新整棵 Room 子树。
- 实时 Store 属于 Thread 私有实现，不从全局 `store/` 暴露协议。
- 桌面与移动端只消费同一个面板模型，不重复补全 Agent 身份或动作能力。
- Thread 根目录只保留目标状态与上下文，不放卡片视图或实时数据投影。
