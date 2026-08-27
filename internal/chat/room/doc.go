// Package room 是 Room 对话领域：公区/定向消息事件、可见性投影与提及解析。
//
// L2 | 父级: internal/chat（L1 见 AGENTS.md）
//
// 成员清单：
//   - mapper.go / events.go / records.go：消息映射、Agent 运行时引用、上下文占用等 Room 作用域事件构建与记录。
//   - context_budget.go / visible_*.go：模型窗口预算、anchor/delta 规划、checkpoint 边界、可见性投影与公区提及增量交付契约。
//   - mention.go / public_mention.go / participation.go / handoff.go / guidance.go / no_reply.go：提及（含已知中英文别名直接衔接汉字正文的最长匹配容错边界）、公区提及、持久成员参与与去重 Agent 成员规模判断、全量非代码 @ handoff、旧 fanout 标记剥离、引导与无回复。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package room
