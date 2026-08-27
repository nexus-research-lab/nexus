// Package channelauthorization 提供 nexus MCP 中 owner-main 私有 DM 的单一 Channel 授权 action 工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - contract/：server 固定 Actor 与窄服务接口。
//   - tool/：承载 start/status/cancel/request-verification-code 的单一严格 action 工具。
//   - server.go：工具定义装配。
//
// QR/device payload 与验证码永不进入 MCP 结果或参数，只经过已认证的原生人类展示路径。
//
// 对外暴露：BuildTools。
//
// [PROTOCOL]: 包契约变更时更新此头部。
package channelauthorization
