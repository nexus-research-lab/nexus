// Package execution 提供当前/历史 WorkGraph 读取、durable Draft/版本编辑、隐藏后台保存调度与命名工作图目录管理 HTTP 边界。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：按 owner/session 返回当前及历史 managed ExecutionView、从 exact 完成图生成/复用 Draft、创建或恢复隐藏编辑会话、选择版本、调度不进入聊天时间线的内部保存 round，并列出或删除已保存工作图；命名图持久化只经 Skill + CLI。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/execution_view.go 与 AGENTS.md（L1）
package execution
