# hooks/agent/transport/

L4 | 父级: ../CLAUDE.md

负责 WebSocket 连接、信封校验、事件路由和流式缓冲。`handlers/` 按消息、权限、重同步、Session 与作用域事件分域；业务处理器通过 `AgentEventContext` 的小接口访问其他层。

未知事件保持忽略，以允许后端先发布不影响旧前端的新事件；非法信封必须记录警告。
生成协议中的 `data` 保持 `unknown`，由事件所有者校验必需字段后再进入运行态。
事件分发器保持稳定的 Socket 回调，并通过 ref 读取当前会话上下文。
每个事件类型只能属于一个处理器映射，重复注册必须在路由表创建时失败。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
