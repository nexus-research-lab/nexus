// Package automation 是定时任务/heartbeat 的服务编排层（调度、执行、投递、观测、CRUD）。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 与 internal/automation 分工：那里是调度域纯逻辑，这里是服务编排与运行时接线。
//
// 成员清单：
//   - task_*.go / runtime_state.go：任务 CRUD、查询、运行与统一运行态投影。
//   - scheduler*.go：到期工作扫描、阶段分发、数据库租约与恢复。
//   - execution*.go：脚本、主会话、独立会话的分阶段执行 / 观测 / 重叠与 misfire 处理。
//   - heartbeat_*.go：heartbeat 输入分段、分发、运行时与状态。
//   - observability*.go / daily_report.go：可观测性、健康与日报。
//   - delivery_retry.go：投递重试。
//   - runtime_*.go：执行工件 / 投递 / 脚本 / 进程运行态。
//   - permission_scheduled.go / summary_heartbeat_tasks.go：定时权限、heartbeat 汇总。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
