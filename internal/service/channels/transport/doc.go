// Package transport 提供通道出站的 HTTP/文本传输底座。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：出站客户端。
//   - http.go：HTTP 传输。
//   - text.go：文本消息传输。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package transport
