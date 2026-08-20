// Package execution 提供当前/历史 WorkGraph 读取与命名 Workflow 目录管理 HTTP 边界。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：按 owner/session 返回当前及历史 managed ExecutionView，并列出或删除只含责任节点语义的命名 Workflow；创建只经 Skill + CLI。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/execution_view.go 与 AGENTS.md（L1）
package execution
