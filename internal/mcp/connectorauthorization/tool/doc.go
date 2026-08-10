// Package tool 把 Connector 授权服务适配成严格、脱密的 MCP 工具。
//
// L2 | 父级: internal/mcp/connectorauthorization
//
// 成员清单：
//   - registry.go：仅 owner-main 私有 DM 可见的工具集合。
//   - start.go / status.go / cancel.go：三操作薄适配。
//   - schema.go / helpers.go：拒绝额外身份/秘密字段的参数与 JSON 结果。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级 doc.go（L2）
package tool
