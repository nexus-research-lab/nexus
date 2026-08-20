// Package execution 提供当前/历史 WorkGraph 读取、非持久化草图预览、隐藏后台保存调度与命名工作图目录管理 HTTP 边界。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：按 owner/session 返回当前及历史 managed ExecutionView、从 exact 完成图生成临时草图、调度不进入聊天时间线的内部保存 round，并列出或删除已保存工作图；持久化只经 Skill + CLI。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/execution_view.go 与 AGENTS.md（L1）
package execution
