// Package relay 提供 Nexus Server 到 Nexus Relay v1 的 typed HTTP/WSS client。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Bearer 认证、幂等消息提交、快照/增量查询、WSS 水位订阅与 upgrade 错误解码。
//   - model.go：Relay M1 bootstrap、message、snapshot、difference 与 stream.updated wire 合同。
//
// 暴露接口：Client、NewClient 及 Relay v1 typed DTO。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package relay
