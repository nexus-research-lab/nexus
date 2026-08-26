// Package toolpolicy 归一化工具名策略集合（nil/空表示无显式策略）。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - toolpolicy.go：工具名集合归一化、规则表匹配与托管工具策略；当前结构化 command 自动审批，历史 CLI 只保留只读轨迹分类。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package toolpolicy
