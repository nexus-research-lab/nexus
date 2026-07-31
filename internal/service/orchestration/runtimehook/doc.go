// Package runtimehook adapts Execution-owned subagent admission and lifecycle
// results to the Agent SDK bridge hook wire contract.
//
// L2 | 父级: internal/service/orchestration（L1 见 AGENTS.md）
//
// 成员清单：
//   - hook.go：宿主 runtime/Room identity 闭包、PreToolUse 准入、Subagent
//     lifecycle 转发与结构化拒绝投影。
//
// [PROTOCOL]: 变更时更新此头部，然后检查上级 orchestration/doc.go（L2）
package runtimehook
