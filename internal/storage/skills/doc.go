// Package skills 是技能来源与导入记录的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / source.go / imported.go：仓储入口、技能来源与导入记录读写。
//   - model_skill.go：SourceEntity 等模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skills
