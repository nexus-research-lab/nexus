// Package auth 提供统一认证：密码登录、Session 签发、Cookie、Principal、Owner 初始化。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / cookie.go / desktop.go / principal.go /
//     session.go / token.go / user.go / validate.go：Service 各切面。
//   - password.go：密码校验/哈希。
//   - model_auth.go / context.go：认证模型与请求上下文（含客户端 IP 解析）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package auth
