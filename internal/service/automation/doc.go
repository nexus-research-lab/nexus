// Package automation 是定时任务/heartbeat 的服务编排层（调度、执行、投递、观测、CRUD）。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 与 internal/automation 分工：那里是调度域纯逻辑，这里是服务编排与运行时接线。
//
// 成员清单：
//   - task_crud.go / task_configuration.go / task_*.go / runtime_state.go：
//     任务创建幂等、配置版本 CAS、到期停用、查询、运行与统一运行态投影；isolated
//     Session 清理由 SessionArtifactDeletionCoordinator 安装 tombstone 后统一回收。
//   - script_control_boundary.go：Agent actor 对 script 任务的 service 级最终拒绝与并发控制。
//   - delivery_authority.go：Agent-origin create/update 与实际投递时的 owner-main/self/Room 动态权限复核。
//   - scheduler.go：到期工作扫描、阶段分发、数据库租约与超时恢复。
//   - execution*.go：脚本、主会话、独立会话的分阶段执行、非交互来源标记 / 观测 / 重叠与 misfire 处理。
//   - heartbeat_*.go：heartbeat 输入分段、分发、运行时与状态。
//   - observability_health.go / observability_util.go / daily_report.go：状态查询、健康计算与日报。
//   - delivery_retry.go：投递重试；重试同样通过最新任务与动态权限复核。
//   - runtime_*.go：执行工件 / 投递 / 脚本 / 进程运行态；desktop 脚本只继承
//     必要系统环境并把 HOME/TEMP 收窄到任务 workspace/临时目录。
//   - permission_scheduled.go / summary_heartbeat_tasks.go：定时权限、heartbeat 汇总。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
