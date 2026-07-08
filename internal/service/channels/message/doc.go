// Package message 把标准入站消息投影为 runtime/history 可持久化的 metadata。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单：
//   - model.go：入站消息与 RuntimeMetadata 模型。
//   - migration.go：消息 metadata 迁移。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package message
