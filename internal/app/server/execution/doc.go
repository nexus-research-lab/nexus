// Package execution 装配 Execution command context、精确取消与 Subagent 历史投影。
//
// L2 | 父级: internal/app/server（L1 见 AGENTS.md）
//
// 成员清单：
//   - command_context.go：从可信 runtime identity 构造 Execution command context。
//   - cancellation.go：把 durable cancellation 路由到 Room slot 或精确 runtime round。
//   - subagent_history.go：把 Session 历史投影为 Runtime Graph 只读事实。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package execution
