// Package logx 提供结构化日志：handler、渲染、配色与滚动落盘。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - logger.go / handler.go / context.go：Logger、slog handler、上下文注入。
//   - pretty.go / render.go / color.go / text.go：美化输出、渲染、ANSI 配色、文本。
//   - rolling.go：滚动落盘。
//   - extract.go / value.go / model.go：字段抽取、取值、模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package logx
