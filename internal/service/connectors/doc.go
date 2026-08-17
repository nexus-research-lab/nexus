// Package connectors 提供连接器目录、OAuth 授权与连接状态能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / listing.go：Service 与目录列举。
//   - connection*.go / configuration_state.go：连接查询、刷新、存储与脱敏配置版本快照。
//   - mutation.go：owner + Connector 串行化、数据库 CAS 与单调配置版本事务。
//   - oauth_*.go / device_flow.go：OAuth 授权、飞书扫码选取/创建应用、
//     用户自有应用配置、手工凭据兜底与 Device Flow。
//   - authorization_*.go：owner-main 私有 DM 的 durable 人工批准、
//     opaque flow、加密 provider 临时秘密、跨 round 恢复与 Connector CAS 完成。
//   - catalog.go / model.go / credential_payload.go：目录、模型与凭据载荷。
//   - custom_mcp.go：owner 级自定义 MCP 的加密配置、目录投影与 runtime 读取。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package connectors
