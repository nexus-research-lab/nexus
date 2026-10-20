// Package teamrelay 持久保存本机节点授权、执行任务与输出 outbox。
//
// L2 | 父级: internal/storage（L1 见 doc.go）
//
// 成员清单：
//   - repository.go：Repository 构造；在线消息以 Relay 为唯一权威，00158 删除旧投影表。
//   - node.go：本机授权意图、加密凭据与精确 Node 状态 CAS；不保存浏览器 Cookie 明文。
//   - jobs.go：本机任务 CAS、单 Agent 活跃约束、启动/撤销共享锁及完整输出 outbox；SaveNodeJob 提交成功后把输出序号写回调用方 job，空 claim 由 DiscardClaimingNodeJob 删除。
//     00140 提供 SQLite/PostgreSQL 表和索引。
//     00141 回填来源 Room/消息/Delivery 索引，旧 Thread 按 owner/scope/Room 精确定位。
//     显式交付文件的字节与去重 ID 原子进入已有 JSON outbox；上传后替换为不可变远端引用，确认前保留原命令。
//
// 暴露接口：Repository、NewRepository 与 node.go/jobs.go 的节点授权和任务方法。
//
// 远端 DTO 消费 internal/relay 合同，不依赖 service/relay 客户端实现。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go
package teamrelay
