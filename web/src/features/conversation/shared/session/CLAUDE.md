# Conversation Session

- `use-conversation-session.ts` 统一编排 DM / Room 的运行时会话、live layout epoch、滚动、历史和时间线。
- 消费者只提供聊天类型、身份和 Room 事件回调，不重复拼接底层 hooks。
- 导航、回到底部和视口投影各自定义最小 Session Source，不得从 Hook 实现推导完整返回类型。
- 会话键只从 `identity.session_key` 派生，不维护第二份输入状态。
- transport 重连后的 durable Session reload 必须继续进入本目录已有的消息合并和 runtime reconciliation；成功对账同时更新 reliability，失败只报告 `session_load_failed` 分类，不把原始 HTTP 错误写进 Feed。
- `visibleAfterUnixMilli` 是嵌入式 Session 的展示边界；它必须在统一 timeline 输入处过滤，不能改变 runtime resume 上下文。WorkGraph 隐藏编辑 Session 不继承来源 transcript，只展示自身创建后的持续编辑消息，并由视图层提供不进入 transcript 的本地接待说明。
- WorkGraph 编辑 Session 必须同时传入 `initialScrollAnchor=top` 与 `liveContentAlignment=start`：首次以接待说明为顶部锚点，短内容向下增长；溢出后的 FOLLOW 与用户上滚后的 READING 仍复用共享状态机。
- live layout epoch 必须合并 runtime phase、live round、Assistant stream、slot 与 execution source；任一可见来源仍活跃时都不能提前结算 Feed 高度回缩。
- 本目录负责会话基础设施，不包含 Goal、Composer、快照或具体视图逻辑。
