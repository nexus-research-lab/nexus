// Package auth 是认证域（用户/会话/密码/状态）的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / repository_user.go / repository_session.go / repository_password.go /
//     repository_state.go / repository_value.go：各认证实体读写。
//   - model_auth.go：认证持久化模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package auth
