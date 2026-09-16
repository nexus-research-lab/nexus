// Package communication 提供 nexus MCP 中统一的 DM 与 Room 通讯工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - im_delivery.go：同一 list_targets/send_message 下的 delivery_sources 查询与 delivery_source 回传封闭参数合同。
//   - server.go：宿主上下文路由、严格 Schema 与覆盖联系人、Room、外部 Session 的 list_targets/send_message 工具。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package communication
