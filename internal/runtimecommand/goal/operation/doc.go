// Package operation 实现 Goal command 的固定语义操作集。
//
// L2 | 父级: internal/runtimecommand/goal（L1 见 AGENTS.md）
//
// 成员清单：
//   - create_goal.go / retarget_goal.go / audit_objective_alignment.go / update_goal.go / get_current.go：带模型侧 readiness gate 的创建、重定向、结构化目标对齐审计、result-first 状态收口、读取当前 Goal。
//   - mutation_authority.go：retarget/audit/update 共用的 exact Goal/revision/Execution runtime capability fence。
//   - registry.go / metadata.go / schema.go / result.go：工具注册、元数据、入参 schema、完整 structured usage、Codex text 结果与最终交付指引构造。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package operation
