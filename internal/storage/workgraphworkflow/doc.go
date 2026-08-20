// Package workgraphworkflow 持久化 owner-scoped 命名 WorkGraph Workflow。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - repository.go：Workflow aggregate 的跨方言事务写入、按 owner/name 读取与删除。
//
// 该存储只接收责任节点语义与依赖，不接收 Runtime Graph、Tool、Attempt、Submission 或 Acceptance。
package workgraphworkflow
