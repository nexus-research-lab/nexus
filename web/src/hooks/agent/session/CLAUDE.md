# hooks/agent/session/

L4 | 父级: ../CLAUDE.md

负责会话身份迁移、消息历史加载、后台缓存和易失快照。请求编号隔离过期响应；运行态只通过 runtime 层提供的小接口同步。

- `controller/` 分离身份切换、后台/易失快照、生命周期上下文和总装配。
- `use-agent-conversation-history.ts` 只持有历史分页状态与加载互斥。
- `conversation-history-model.ts` 统一 Room/Session 历史源选择和请求规划，加载器只执行副作用。
- `use-agent-conversation-session.ts` 只暴露开始、加载、绑定、清空和重置命令。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
