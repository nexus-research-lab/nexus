// Package execution 提供当前会话 WorkGraph 的只读 HTTP 投影。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：按 owner/session 返回当前或最近一次 managed ExecutionView；普通 runtime-only round 不进入公共 WorkGraph 读取面。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/execution_view.go 与 AGENTS.md（L1）
package execution
