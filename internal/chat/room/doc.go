// Package room 是 Room 对话领域：公区/定向消息事件、可见性投影与提及解析。
//
// L2 | 父级: internal/chat（L1 见 AGENTS.md）
//
// 成员清单：
//   - mapper.go / events.go / records.go：消息映射、事件构建、记录。
//   - context_budget.go / visible_*.go：模型窗口预算、anchor/delta 规划、checkpoint 边界、可见性投影与公区提及增量交付契约。
//   - mention.go / public_mention.go / guidance.go / no_reply.go：提及、公区提及、引导、无回复。
//   - model_runtime.go：Room 运行时模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
