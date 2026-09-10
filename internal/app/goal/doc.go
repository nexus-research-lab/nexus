// Package goal 装配 Goal 命令、会话所有权、运行时续跑。
//
// L2 | 父级: internal/app（L1 见 AGENTS.md）
//
// 成员清单：
//   - command.go：Goal 命令路由、round-scoped authority 与 Agent/Room 会话所有权证明。
//   - lifecycle.go：运行时引导、精确中断与 durable resume。
//
// Goal/Execution 的业务协调由 service/goalexecution 提供。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 ../doc.go（L2）
package goal
