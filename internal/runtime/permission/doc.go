// Package permission 处理 runtime 工具权限请求的呈现与 WebSocket 交互。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - request.go：权限请求模型。
//   - presenter.go：请求呈现。
//   - context.go：Sender 等 WS 事件发送抽象与上下文。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package permission
