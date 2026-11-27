# hooks/agent/session/

L4 | 父级: ../CLAUDE.md

负责会话身份迁移、消息历史加载、后台缓存和易失快照。请求编号隔离过期响应；运行态只通过 runtime 层提供的小接口同步。

- `controller/` 分离身份切换、后台/易失快照、生命周期上下文和总装配。
- `use-agent-conversation-history.ts` 只持有历史分页状态与加载互斥；向前游标、已确认的会话起点和 `has_more` 通过 `commitHistoryCursor` 一次提交，向后加载与回到最新只在窗口与最新内容之间存在 gap 时可用。
- `conversation-history-model.ts` 统一 Room/Session 历史源选择和请求规划，加载器只执行副作用；服务端游标不再指向驻留头部（淘汰或跳转之后）时按驻留头部 round identity 请求，会话起点仍驻留时停止向前请求。
- Room 与 Session API 返回同一完整分页契约，生命周期和历史加载器不得再声明可选字段兼容页。
- `use-agent-conversation-session.ts` 只暴露开始、加载、绑定、清空和重置命令；开始/新建 Session 取消普通页面级 ACK，但保留显式标记的 Goal durable owner；明确 clear/reset 取消全部请求。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
