// Package execution 提供当前/历史 WorkGraph 读取、durable Draft/版本编辑、用户确认后的直接事务保存与命名工作图目录管理 HTTP 边界。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：按 owner/session 返回当前、exact 及历史 managed ExecutionView、从 exact 完成图生成/复用 Draft、检查命名 Slash 可用性、创建或恢复隐藏编辑会话、选择版本、直接保存用户确认的草图和表单元信息，并列出或删除已保存工作图；Apply 的失败事实只使用现有 editor/revision 语义，UI 和模型 command 共用草图与命名图的事务边界。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/execution_view.go 与 AGENTS.md（L1）
package execution
