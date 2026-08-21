// Package browser 提供 Nexus 浏览器扩展的状态与 WebSocket transport。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handler.go：连接状态、固定扩展来源与子协议校验、握手、命令回执读取和连接生命周期。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package browser
