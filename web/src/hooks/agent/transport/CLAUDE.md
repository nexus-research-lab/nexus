# hooks/agent/transport/

L4 | 父级: ../CLAUDE.md

负责 WebSocket 连接、信封校验、事件路由和流式缓冲。路由表只选择处理器；业务处理器通过 `AgentEventContext` 的小接口访问其他层。

未知事件保持忽略，以允许后端先发布不影响旧前端的新事件；非法信封必须记录警告。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
