// Package tool 把 Connector 授权服务适配成单一、严格且脱密的 action MCP 工具。
//
// L2 | 父级: internal/mcp/connectorauthorization
//
// 成员清单：
//   - registry.go：仅 owner-main 私有 DM 可见的 start/status/cancel action 适配。
//   - schema.go / helpers.go：拒绝额外身份/秘密字段的参数与 JSON 结果。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级 doc.go（L2）
package tool
