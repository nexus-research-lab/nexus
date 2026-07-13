// Package roomrepo 是房间的跨方言 SQL 仓储（创建/加载/删除）。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - sql.go / sql_load.go / sql_delete.go：房间读写、删除计划与事务执行。
//   - model.go / scan.go：房间模型与行扫描。
//
// SQLRepository 根据 driver 选择 SQLDialect，不在上层复制 SQLite/PostgreSQL 门面。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package roomrepo
