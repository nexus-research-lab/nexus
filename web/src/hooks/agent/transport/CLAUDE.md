# hooks/agent/transport/

L4 | 父级: ../CLAUDE.md

负责 WebSocket 连接、信封校验、事件路由和流式缓冲。`handlers/` 按消息、权限、重同步、Session 与作用域事件分域；业务处理器通过 `AgentEventContext` 的小接口访问其他层。完整 `message` 快照提交前必须同步 flush 同一连接已排队的 stream patch，维持 WebSocket 到达顺序；每次 flush 捕获最新活动 `session_key` 并作精确匹配，消息状态真正提交时还必须确认会话没有再次切换，禁止旧 RAF patch 越过会话切换或新快照。

未知事件保持忽略，以允许后端先发布不影响旧前端的新事件；非法信封必须记录警告。
生成协议中的 `data` 保持 `unknown`，由事件所有者校验必需字段后再进入运行态。
事件分发器保持稳定的 Socket 回调，并通过 ref 读取当前会话上下文。
每个事件类型只能属于一个处理器映射，重复注册必须在路由表创建时失败。
流式载荷只按 `requestAnimationFrame` 合并同一可见帧；合并后的消息写入保持普通优先级，禁止再包 `startTransition` 延迟中间进度。同帧 `message_start + content delta` 的首个 text/thinking 必须先以本地 reveal 标记提交，下一 RAF 再清标记；完整 snapshot 同任务到达时沿用同一边界。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
