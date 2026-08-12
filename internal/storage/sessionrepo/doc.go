// Package sessionrepo 提供 Room Session 视图的跨方言 SQL 查询。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - sql.go：只读 Room Session 视图查询与遵循 Room-first 锁协议的 SDK session ID 回写；
//     Room conversation 生命周期和版本仍归 roomrepo，不得并入 workspace Session CAS；
//     SQL messages 计数仅是 legacy import 下限，实时进度由 service 与 workspace 投影合并。
//
// SQLRepository 根据 driver 选择 SQLDialect，不在上层复制 SQLite/PostgreSQL 门面。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package sessionrepo
