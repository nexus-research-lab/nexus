# Room 页面模型

- 会话、成员、Session 身份和快照写回各自使用独立纯模型；Hook 只负责缓存及外部 Session 资源。
- `room-session-model.ts` 通过 DM 与 Group 策略构造 Session 身份，不在页面或视图中拼接 Session Key。
- `room-snapshot-model.ts` 统一解释联合快照的显式字段与当前作用域回退；`use-room-conversation-snapshot.ts` 只执行投影后的状态写回和目录通知。
- `page/` 分阶段组合基础 Room 投影、外部 Session 和最终页面模型。
- 相同协议字段只在模型中解释一次，视图不得重新推导 Session 键或活动顺序。
- 快照写回必须同时匹配当前 Room、Conversation 和 Session 作用域。
- Room 根路由按“活动项、其余已打开项”的顺序恢复持久化标签栏；显式 Conversation 路由优先，失效活动项先回退到仍有效的打开标签，全部失效才使用当前资源顺序首项。
- 最后激活项属于外部 Session 时，首次恢复必须等待当前 Agent 的外部 Session 目录完成一次加载，再确认目标或回退，禁止先打开普通会话并覆盖恢复偏好。
