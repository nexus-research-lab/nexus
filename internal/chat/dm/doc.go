// Package dm 是 DM 对话领域：把 SDK 消息映射为 DM 协议事件与持久消息。
//
// L2 | 父级: internal/chat（L1 见 AGENTS.md）
//
// 成员清单：
//   - mapper.go：MessageMapper SDK 消息 → DM 事件/持久消息。
//   - messages.go / session.go：消息模型与会话领域。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package dm
