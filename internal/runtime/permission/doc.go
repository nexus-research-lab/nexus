// Package permission 处理 runtime 阻塞式人工交互的呈现与 WebSocket 响应。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - request.go：等待用户响应、按过期时间和请求身份稳定重放的请求模型。
//   - presenter.go：批准、问答及未知工具的兼容呈现。
//   - context.go：Sender 等 WS 事件发送抽象与上下文。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package permission
