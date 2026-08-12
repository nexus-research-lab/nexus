// Package contract 定义通道无关的投递契约（DeliveryTarget 等）。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单：
//   - model.go：投递目标与 Automation run 投影契约；区分稳定 context_token 与即时 callback req/stream。
//   - util.go：契约辅助。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package contract
