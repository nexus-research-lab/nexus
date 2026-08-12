// Package communication 提供 Agent 平台通讯录与跨当前会话的消息发送能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：runtime Actor 重校验、通讯录投影、联系人私信 Room 复用与群消息发送。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package communication
