// Package shared 提供 handler 层共享的 HTTP 响应、中间件、上下文与 WS 发送能力。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - api.go：统一 HTTP 响应与上下文辅助。
//   - etag.go：条件写使用的强 ETag 写出与 If-Match 解析。
//   - failure.go / request_context.go：可选 FailureCore 写出与仅供诊断的 HTTP request ID 上下文。
//   - middleware.go：请求中间件。
//   - sender_websocket.go：WebSocket 发送器。
//   - desktop_session.go：桌面会话辅助。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package shared
