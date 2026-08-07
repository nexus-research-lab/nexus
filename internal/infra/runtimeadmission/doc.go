// Package runtimeadmission serializes Agent runtime admission with security-boundary transitions.
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - gate.go：并发 admission lease、转场阻断、在途启动撤销与 drain。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtimeadmission
