# Conversation 基础协议

本目录提供不属于具体页面 Feature 的会话标识、外部通道和导航纯协议。

- `external-session.ts` 统一外部通道别名、标签、合成会话 ID 与会话投影。
- `session-key.ts` 只解析稳定 Session Key 协议。
- `direct-room-navigation.ts` 和 `room-conversation-delete.ts` 提供跨 Feature 共用的窄命令/规则。
- 基础协议可以依赖 `types/`，不得依赖 `features/` 或 React 视图。
