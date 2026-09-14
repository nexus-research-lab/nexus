// Package relay 定义 Nexus 消费 Relay 的独立跨仓合同。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：model.go 定义 Room、消息、提交回执、快照、增量、水位和远端错误。
// 客户端、Team 同步服务和本地投影共享这些类型，本包不依赖其他 internal 包。
//
// [PROTOCOL]: 合同变更时检查 service/relay、service/team、storage/teamrelay 与父级 AGENTS.md。
package relay
