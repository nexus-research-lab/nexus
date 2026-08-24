// Package workgraphworkflow 持久化 owner-scoped WorkGraph Draft、版本历史与命名图沉淀。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：沉淀 aggregate 的跨方言事务写入、按 owner/name 读取与删除。
//   - draft_repository.go：按 exact source 去重的 Draft、不可变完整版本、head/selected revision、隐藏编辑 Session 绑定与保存状态。
//
// 两类存储都只接收责任节点语义与依赖，不接收 Runtime Graph、Tool、Attempt、Submission 或 Acceptance。
package workgraphworkflow
