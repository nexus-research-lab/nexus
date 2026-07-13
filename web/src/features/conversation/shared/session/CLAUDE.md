# Conversation Session

- `use-conversation-session.ts` 统一编排 DM / Room 的运行时会话、滚动、历史和时间线。
- 消费者只提供聊天类型、身份和 Room 事件回调，不重复拼接底层 hooks。
- 导航、回到底部和视口投影各自定义最小 Session Source，不得从 Hook 实现推导完整返回类型。
- 会话键只从 `identity.session_key` 派生，不维护第二份输入状态。
- 本目录负责会话基础设施，不包含 Goal、Composer、快照或具体视图逻辑。
