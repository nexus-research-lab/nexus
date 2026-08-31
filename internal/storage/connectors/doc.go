// Package connectors 是连接器 OAuth 凭据的 SQL 存储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - oauth_client.go：OAuthClient 模型、应用凭据读写及调用方事务复用入口。
//   - authorization_flow.go / authorization_model.go：人工批准的
//     OAuth/Device flow、加密 attempt 凭据、不改 canonical 配置的阶段推进与事务终态。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package connectors
