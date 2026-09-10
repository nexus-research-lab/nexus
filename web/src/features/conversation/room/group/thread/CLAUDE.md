# Room Group Thread

## 职责

- `group-thread-state.ts` 与 `group-thread-context.tsx` 只维护精确到 `agent_round_id` 的当前 Thread 目标和开关命令。
- `live/` 独占实时会话切片、纯面板投影与生产消费 Hook。
- `round-card/` 独占主 Feed 的轮次卡片投影与视图。

## 边界

- 控制上下文不承载消息、权限或回调，避免实时流更新整棵 Room 子树。
- 实时 Store 属于 Thread 私有实现，不从全局 `store/` 暴露协议。
- 桌面与移动端只消费同一个面板模型，不重复补全 Agent 身份或动作能力。
- Thread 根目录只保留目标状态与上下文，不放卡片视图或实时数据投影。
- 停止动作属于主 Feed 的 Agent slot 卡片；Thread 面板和通用消息项不暴露 Room 全局停止回调。
- 控制 Provider 保留轻量选择态：重复打开同一 root/Agent/agent_round 不替换目标，目录/语言/普通回调刷新不关闭；会话切换的关闭与 source 发布/卸载清理继续由 `live/use-room-thread-source.ts` 统一驱动，不能在 Header、窄窗或 Provider 再造重置缓存。共置 DOM 回归使用真实发布者与独立消费叶子验证 A→B→A 不复活旧选择，保持聊天子树挂载。
