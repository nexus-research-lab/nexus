// Package teamrelay 把 Relay 的共享消息投影到 Nexus 本地数据库。
//
// L2 | 父级: internal/storage（L1 见 doc.go）
//
// 成员清单：
//   - repository.go：deployment 共享 Conversation/Message、owner 独立 cursor 的事务投影。
//   - node.go：本机授权意图、加密凭据与精确 Node 状态 CAS；不保存浏览器 Cookie 明文。
//   - jobs.go：本机任务 CAS、单 Agent 活跃约束、启动/撤销共享锁及完整输出 outbox；00140 提供 SQLite/PostgreSQL 表和索引。
//     00141 回填来源 Room/消息/Delivery 索引，旧 Thread 按 owner/scope/Room 精确定位。
//     显式交付文件的字节与去重 ID 原子进入已有 JSON outbox；上传后替换为不可变远端引用，确认前保留原命令。
//
// 暴露接口：Repository、NewRepository、ProjectRoom、ProjectCommit、ProjectSnapshot、ProjectDifference。
//
// 远端 DTO 消费 internal/relay 合同，不依赖 service/relay 客户端实现。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go
package teamrelay
