# Room 页面模型

- 会话、成员、Session 身份和快照写回各自使用独立纯模型；Hook 只负责缓存及外部 Session 资源。
- `room-session-model.ts` 通过 DM 与 Group 策略构造 Session 身份，不在页面或视图中拼接 Session Key。
- `room-snapshot-model.ts` 统一解释联合快照的显式字段与当前作用域回退；`use-room-conversation-snapshot.ts` 只执行投影后的状态写回和目录通知。
- `page/` 分阶段组合基础 Room 投影、外部 Session 和最终页面模型。
- 相同协议字段只在模型中解释一次，视图不得重新推导 Session 键或活动顺序。
- 快照写回必须同时匹配当前 Room、Conversation 和 Session 作用域。
- 服务端 `is_draft` 是未开始状态的真相；消息观察器只在显式 `has_user_input=true` 时把本地 `is_draft` 收敛为 `false`。局部 `message_count` 只能单调提升计数，不得参与草稿判断、降低服务端计数或把已开始会话恢复成草稿。
- Room 根路由按“活动项、其余已打开项”的顺序恢复持久化标签栏；显式 Conversation 路由优先，失效活动项先回退到仍有效的打开标签，全部失效才使用当前资源顺序首项。
- 最后激活项属于外部 Session 时，首次恢复必须等待当前 Agent 的外部 Session 目录完成一次加载，再确认目标或回退，禁止先打开普通会话并覆盖恢复偏好。
- 外部 Session 目录由共享目录事件、窗口重新聚焦和恢复可见驱动刷新，不做固定间隔轮询。
- 外部 Session 快照按 exact Room + Agent 隔离；普通刷新失败保留同 scope 最后成功标签并标为 stale，Room/Agent 变化或权限失效必须清空，恢复只由现有目录/窗口可见性事件或用户显式刷新触发。
