# Agent Session Controller

- `use-agent-session-controller.ts` 只装配身份、历史、快照、生命周期和公开会话命令。
- `use-agent-session-identity.ts` 管理身份切换、游标归零、ACK 取消和当前会话事件判定。
- `use-agent-session-snapshots.ts` 管理后台消息缓存与易失会话快照的恢复、持久化。
- `use-agent-session-lifecycle-context.ts` 只组装会话加载函数消费的窄上下文。
- 原生 React setter 直接作为状态命令使用，不增加无行为的镜像包装。
