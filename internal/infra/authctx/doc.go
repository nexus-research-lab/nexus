// Package authctx 在请求上下文里读写认证主体。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - context.go：认证主体模型、请求上下文、人类在场证据、durable queue 人类绑定，以及本地单用户 Web 控制面判定。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package authctx
