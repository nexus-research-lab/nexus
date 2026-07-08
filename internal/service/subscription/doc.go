// Package subscription 在账号达到月度 token 额度后阻止新的 runtime 请求。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：EnsureQuotaAvailable 额度校验。
//   - model_subscription.go：订阅模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package subscription
