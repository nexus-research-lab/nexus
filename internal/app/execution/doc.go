// Package execution 装配 Execution command context、精确取消与 Subagent 历史投影。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - recovery.go：把 orchestration 自动/用户节点恢复请求和回执读取路由到 DM/Room 的持久队列。
//   - command_context.go：从可信 runtime identity 构造 Execution command context。
//   - cancellation.go：把 durable cancellation 路由到 Room slot 或精确 runtime round。
//
//   - scheduled_dispatch.go：连接 DM/Room 队列与 Goal 当前状态准入，路由审核回交。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 ../doc.go（L2）

package execution
