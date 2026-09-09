// Package goalexecution 协调 Goal 与 Execution 的显式绑定、目标修订和晋升。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - execution.go：独立 Goal 创建、绑定预检与确认、可恢复的目标修订协调及 Goal 操作转发。
//   - promotion.go：Execution 晋升为 Goal 的幂等协调与完成前的执行审计适配。
//
// 本包消费 Goal/Orchestration 的领域端口，不直接依赖 HTTP、MCP 或 app 装配。
// Goal 持有修订与持久阶段，Orchestration 持有执行状态；本包只协调跨域顺序。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package goalexecution
