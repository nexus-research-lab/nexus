// Package toolpolicy 归一化工具名策略集合（nil/空表示无显式策略）。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - toolpolicy.go：工具名集合归一化、规则表匹配与托管工具策略；Goal/Execution/Automation CLI 只允许无 shell 后处理的单进程调用。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package toolpolicy
