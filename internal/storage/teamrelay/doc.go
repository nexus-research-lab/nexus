// Package teamrelay 把 Relay 的共享消息投影到 Nexus 本地数据库。
//
// L2 | 父级: internal/storage（L1 见 doc.go）
//
// 成员清单：
//   - repository.go：deployment 共享 Conversation/Message、owner 独立 cursor 的事务投影。
//
// 暴露接口：Repository、NewRepository、ProjectBootstrap、ProjectCommit、ProjectSnapshot、ProjectDifference。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go
package teamrelay
