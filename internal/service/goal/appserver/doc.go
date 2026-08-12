// Package appserver 定义 Codex app-server 协议里的 Goal 状态与 RPC 模型。
//
// L2 | 父级: internal/service/goal（L1 见 AGENTS.md）
//
// 成员清单：
//   - model_goal.go：ThreadGoalStatus 与 actual/budget token 双口径 Goal 状态表示。
//   - model_rpc.go：thread/goal RPC 模型，以及 conflict server-error code 与稳定 reason_code。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package appserver
