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
//   - delivery_authority.go：创建 provenance 与独立 delivery grant 分离，以及 create/update/投递/重试时对真实 Nexus/Room/IM Session、Room 回复 Agent、owner-main/self/成员/active pairing 的动态复核；Room 未显式选择回复者时固化当前房主。
//   - scheduler.go：到期工作扫描、阶段分发、数据库租约与超时恢复。
//   - execution*.go / main_session_execution.go：脚本、主会话、独立会话的分阶段执行、非交互来源标记、物理 attempt 收尾屏障、权限续跑证据、观测、重叠与 misfire 处理。
//   - heartbeat_*.go：heartbeat 输入分段、分发、运行时与状态。
//   - observability_health.go / observability_util.go / daily_report.go：状态查询、健康计算与日报。
//   - delivery_retry.go：投递重试；重试同样通过最新任务与动态权限复核。
//   - runtime_*.go：执行工件 / 投递 / 脚本 / 进程运行态；desktop 脚本只继承
//     必要系统环境并把 HOME/TEMP 收窄到任务 workspace/临时目录。
//   - permission_snapshot.go / permission_policy.go / permission_scheduled.go / permission_decision.go / permission_recipient.go / permission_session.go：执行 Session/Agent 权限 copy-on-create、独立任务授权策略、运行时拦截、持久决策与安全恢复，并把待确认请求按 run 冻结接收目标投影及重放到 Nexus DM/Room/IM Session；没有接收目标时可冻结来源 Session 作为审批兜底。
//   - permission_im_command.go / im_notification.go：只在会话内唯一请求上解析的无 ID `/y`、`/a`、`/d` IM 审批，以及面向 Nexus Session 与外部 IM、带任务身份的控制面通知；普通结果保持原文。
//   - task_support.go：任务容量与 isolated Session 清理；Session 删除时保留任务、停用调度并持久标记待重绑，创建来源只保留 provenance。
//   - summary_heartbeat_tasks.go：heartbeat 汇总。
//   - command_*.go / runtime_command_capability.go：Agent-facing nexus CLI 的严格
//     contract/inspect/plan/apply、当前会话查询、跨 Agent/Session 路由、配置/run 双重
//     revision/digest、durable command replay 与只在唯一活跃 physical round 生效的 capability。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
