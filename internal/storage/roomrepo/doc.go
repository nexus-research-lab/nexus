// Package roomrepo 是房间的跨方言 SQL 仓储（创建/加载/删除）。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - sql.go / sql_load.go / sql_delete.go / sql_draft.go / sql_references.go / sql_participation.go：房间读写、联系人内部通道目录隔离与好友对恢复、每 Room 唯一 draft 的原子确保与修复、fork pending 依赖原子创建及 SDK 物化后的 source transcript 清理所有权转移、至少保留一条对话的删除计划、持久引用探测、带配置版本和权限世代推进的成员参与闸门与事务执行。
//   - model.go / scan.go：含内部用途标记的房间模型与行扫描。
//
// SQLRepository 根据 driver 选择 SQLDialect，不在上层复制 SQLite/PostgreSQL 门面。
// messages 表仅保留 legacy import 与删除引用兼容；当前 Room/DM 历史位于 owner-scoped ledger，服务读模型不能把 COUNT(messages) 当作唯一实时真相。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package roomrepo
