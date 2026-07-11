// Package connectors 提供连接器目录、OAuth 授权与连接状态能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / listing.go：Service 与目录列举。
//   - connection*.go：连接查询、刷新、存储。
//   - oauth_*.go / device_flow.go：OAuth 授权、用户自有应用配置、Device Flow。
//   - catalog.go / model.go / credential_payload.go：目录、模型与凭据载荷。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package connectors
