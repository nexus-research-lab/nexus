// Package titlegen 按首条用户消息异步生成会话标题。
//
// L2 | 父级: internal/service/conversation（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：Service 生成编排。
//   - request.go / contract.go：请求构造与契约（关闭 think、max_tokens 等）。
//   - generation.go / title_rules.go：生成主逻辑与标题清洗规则。
//   - apply.go / preview.go：落库与预览填充。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package titlegen
