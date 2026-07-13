# Room 页面模型

- 会话、成员、Session 身份和快照写回各自使用独立纯模型；Hook 只负责缓存及外部 Session 资源。
- `room-session-model.ts` 通过 DM 与 Group 策略构造 Session 身份，不在页面或视图中拼接 Session Key。
- `room-snapshot-model.ts` 统一解释联合快照的显式字段与当前作用域回退；`use-room-conversation-snapshot.ts` 只执行投影后的状态写回和目录通知。
- `page/` 分阶段组合基础 Room 投影、外部 Session 和最终页面模型。
- 相同协议字段只在模型中解释一次，视图不得重新推导 Session 键或活动顺序。
- 快照写回必须同时匹配当前 Room、Conversation 和 Session 作用域。
