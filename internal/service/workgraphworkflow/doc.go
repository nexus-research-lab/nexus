// Package workgraphworkflow 从历史 managed Execution 提炼可复用的命名 WorkGraph Workflow。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：供受管 Execution CLI 使用的 owner-scoped 幂等创建、读取/删除、Slash descriptor 投影与 runtime prompt 展开。
//
// Workflow 只复制 Skill 显式选择的 Work Item 语义契约和内部 DAG；运行、工具与交付历史保持在源 Execution。
package workgraphworkflow
