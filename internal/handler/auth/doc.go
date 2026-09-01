// Package auth 封装认证域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及登录/登出/校验路由。
//   - profile.go：用户资料、密码修改与 exact request 终态回执核对/放弃 handler。
//   - failure.go：个人设置 mutation 的 FailureCore 证据投影。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package auth
