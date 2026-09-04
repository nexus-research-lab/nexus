// Package subscription 按 Control entitlement 投影限制绑定账号；本地主体不进入订阅域。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：Control entitlement 本地投影读取、Nexus 用量聚合与 EnsureQuotaAvailable 额度校验。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package subscription
