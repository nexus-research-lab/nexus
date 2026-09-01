// Package auth 是认证域（用户/会话/密码/状态）的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储入口、事务边界与共享 SQL 值转换。
//   - user.go / session.go / password.go / state.go：各认证实体读写；密码修改以旧 hash 做 CAS，并以互斥 exact request 终态回执提交或阻止迟到写入。
//   - model.go：认证持久化模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package auth
