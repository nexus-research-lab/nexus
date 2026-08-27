// Package goal 装配 Goal 命令、会话所有权、运行时续跑及 Goal/Execution 绑定。
//
// L2 | 父级: internal/app/server（L1 见 AGENTS.md）
//
// 成员清单：
//   - command.go：Goal 命令路由、round-scoped authority 与 Agent/Room 会话所有权证明。
//   - lifecycle.go：运行时引导、精确中断与 durable resume。
//   - execution.go / promotion.go：Goal 与 Execution 的显式绑定、retarget saga 与晋升。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package goal
