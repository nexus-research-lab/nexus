// Package logx 提供结构化日志：handler、渲染、配色与滚动落盘。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - logger.go / handler.go：Logger、slog handler、上下文注入与文本预览。
//   - pretty.go / render.go / color.go：美化输出、渲染、ANSI 配色与日志模型。
//   - rolling.go：滚动落盘。
//   - extract.go / value.go：候选字段抽取与取值。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package logx
