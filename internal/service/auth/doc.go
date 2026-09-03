// Package auth 消费 Desktop 本地主体或外部 Control 签发的 Principal。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - authority.go / model.go / context.go：最小认证权威合同、Principal 与请求上下文。
//   - local_authority.go / owner_projection.go：Desktop 无密码本地主体与 owner 资料投影。
//   - control_client.go / control_client_user.go / control_wire.go：Control HTTP adapter、签名 Principal 验证与人类 Session 核验。
//   - control_binding.go / control_invalidation.go：Control 身份到本地 owner 的确定性绑定与失效消费。
//   - runtime_admission.go：Control 状态核对与 Agent runtime admission 的安全边界。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package auth
