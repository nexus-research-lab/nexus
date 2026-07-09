// Package roomrepo 是房间的 SQL 仓储（创建/加载/删除）。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository_sql.go / repository_sql_load.go / repository_sql_delete.go：房间读写与删除。
//   - model_room.go / scan_room.go：房间模型与行扫描。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package roomrepo
