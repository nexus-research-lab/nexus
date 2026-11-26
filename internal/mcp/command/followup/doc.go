// Package followup 定义主窗口跟进的 nexus.command 操作：列出、设定与取消 agent 写给未来自己的跟进。
//
// L3 | 父级: internal/mcp/command
//
// 成员清单：
//   - registry.go：list_follow_ups（inspect）、schedule_follow_up、cancel_follow_up 与结果投影。
//
// 领域规则（只能在主窗口设、5 分钟到 7 天、数量上限）全部由 dm service 执行；这里只做输入映射。
package followup
