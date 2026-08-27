// Package skills 是技能来源与导入记录的 SQL 仓储。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go / source.go / imported.go：仓储入口及 DB/transaction 共用的来源、用户私有来源凭据元数据与可更新导入记录 SQL。
//   - mutation.go：owner catalog version 根、跨进程 transaction 锁、expected version CAS，以及来源增删改与 version 的同事务提交。
//   - model.go：SourceEntity 等模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skills
