// Package sqlite 提供 SQLite 仓储骨架（Agent/Room/Session）。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：仓储骨架与装配。
//   - repository_agent.go / repository_room.go / repository_session.go：各域 SQLite 实现。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package sqlite
