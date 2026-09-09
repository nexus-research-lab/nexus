// Package relay 提供 Nexus Server 到 Nexus Relay v1 的 typed HTTP/WSS client。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Bearer 认证、在线 Room 创建/列表、幂等消息提交、快照/增量查询、WSS 水位订阅与 upgrade 错误解码。
//   - wire 合同统一位于 internal/relay，本包只处理远端传输。
//
// 暴露接口：Client、NewClient；方法消费 internal/relay 的独立合同。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package relay
