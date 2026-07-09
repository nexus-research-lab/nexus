// Package automation 是定时任务/heartbeat 的服务编排层（调度、执行、投递、观测、CRUD）。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 与 internal/automation 分工：那里是调度域纯逻辑，这里是服务编排与运行时接线。
//
// 成员清单：
//   - service_task_*.go：任务 CRUD / 查询 / 运行 / 历史 / 事件 / 支撑。
//   - service_scheduler*.go：调度与恢复。
//   - service_execution*.go：执行分发 / 观测 / 重叠处理。
//   - service_heartbeat_*.go：heartbeat 分发 / 运行时 / 状态。
//   - service_observability*.go：可观测性、日报、健康。
//   - service_delivery_retry.go：投递重试。
//   - runtime_*.go：执行工件 / 投递 / 脚本 / 进程运行态。
//   - permission_scheduled.go / summary_heartbeat_tasks.go：定时权限、heartbeat 汇总。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
