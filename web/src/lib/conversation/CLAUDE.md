# Conversation 基础协议

本目录提供不属于具体页面 Feature 的会话标识、外部通道和跨消费者失效通知。

- `external-session.ts` 统一外部通道别名、标签、合成会话 ID 与会话投影。
- `session-key.ts` 只解析稳定 Session Key 协议。
- `room-directory-events.ts` 只广播 Room/DM 目录快照失效，不解释刷新策略。
- 仅由单一 Feature 消费的展示能力规则必须归还所属 Feature，不得以通用 helper 名义放入基础协议。
- 基础协议可以依赖 `types/` 和浏览器事件，不得依赖 `features/`、Store 或 React 视图。
