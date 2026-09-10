// Package browser 提供 Nexus Browser 扩展的 HTTP/WebSocket transport。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handler.go：固定扩展 Origin 与协议握手、可取消写入、精确连接身份下的回执/进度/健康/标签事件转交。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package browser
