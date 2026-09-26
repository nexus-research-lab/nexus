// Package transport 提供通道出站的 HTTP/文本传输底座。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：出站客户端。
//   - http.go：HTTP 传输及结构化拒绝；仅对明确 429 且平台等待不超过 30 秒的请求重试至多两次，网络/5xx 不重试。
//   - text.go：文本消息传输。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package transport
