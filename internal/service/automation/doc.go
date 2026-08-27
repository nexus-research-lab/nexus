// Package automation 是定时任务/heartbeat 的服务编排层（调度、执行、投递、观测、CRUD）。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 与 internal/automation 分工：那里是调度域纯逻辑，这里是服务编排与运行时接线。
//
// 成员清单：
//   - task_crud.go / task_configuration.go / task_*.go / runtime_state.go：
//     任务创建幂等、配置版本 CAS、纯查询投影、运行与统一运行态投影；删除先持久
//     claim 并拒绝新操作，再以 exact token 幂等清理全部 run/权限/投递和 isolated
//     Session，最终事务原子写审计并删除定义，scheduler 可恢复中断的清理；跨实例
//     execution ownership 无法证明时持久进入 review_required，禁止按超时猜测执行已停止；
//     owner 可按当前 configuration_version 显式确认原实例已停止，服务端再用私有 token
//     原子收口 ledger 后删除，确认路径不恢复执行或投递。带版本的立即运行和权限恢复
//     把调用方已见版本一直带到 durable runtime claim。
//   - script_control_boundary.go：Agent actor 对 script 任务的 service 级最终拒绝与并发控制。
//   - delivery_authority.go：创建 provenance 与独立 delivery grant 分离，以及 create/update/投递/重试时对真实 Nexus/Room/IM Session、Room 回复 Agent、owner-main/self/成员/active pairing 的动态复核；Room 未显式选择回复者时固化当前房主。
//   - scheduler.go：基于内存最近 deadline、持久失败投递 deadline、变更唤醒与
//     低频审计的阶段分发、数据库 leader 租约、过期/单次完成任务的窄字段停用 CAS、
//     多实例目录收敛与超时恢复；不做每秒扫描。
//   - execution*.go / main_session_execution.go / run_terminal_delivery.go：脚本、主会话、独立会话的分阶段执行、非交互来源标记、物理 attempt 收尾屏障、权限续跑证据、观测、重叠与 misfire 处理；task runtime claim 与首条 run ledger 按 exact owner/job/run/configuration/permission snapshot 原子受理，commit 后才 dispatch，人工 request 可重放 exact ExecutionResult；启动、删除、恢复和投递仅占用同任务分片 fence，不让 runtime dispatch 阻塞其他任务配置；execution terminal 与 exact task runtime 先原子提交，成功结果以 pending ledger 进入可恢复首次投递，删除态只保存 suppressed terminal 且不外投。
//   - heartbeat_*.go：heartbeat 配置与 wake 控制；每次 wake（包括无文本）先在
//     durable configuration_version 事务栅栏内写 outbox，再由 exact claim 租约派发；
//     command request/intent 可重放同一受理回执；启动恢复尚未领取的 wake，已开始但
//     claim 超时的结果按 unknown fail closed、绝不自动重投，控制锁不跨 runtime dispatch。
//   - observability_health.go / observability_util.go / daily_report.go：状态查询、健康计算与日报；retrying 投递单独表达“结果未知、先核对”，不混成可普通重投的 failed。
//   - delivery_retry.go：由最早 next-attempt deadline 驱动的投递重试；重试同样通过最新任务与动态权限复核，并在外投前 durable claim；人工重投可以配置版本+已见 attempts 防止过期页面二次外投，未确认的 retrying 不自动重放，只允许用户核对后的 exact 显式重投。
//   - runtime_*.go：执行工件 / 投递 / 脚本 / 进程运行态；desktop 脚本只继承
//     必要系统环境并把 HOME/TEMP 收窄到任务 workspace/临时目录；exact script attempt
//     cancel/drain 通过 Unix 独立进程组或 Windows Job Object 防止删除提交早于
//     整棵本地脚本进程树停止；无法证明已停止时保持 review_required。
//   - permission_snapshot.go / permission_policy.go / permission_scheduled.go / permission_decision.go / permission_recipient.go / permission_session.go：执行 Session/Agent 权限 copy-on-create、独立任务授权策略、启动期旧策略幂等回填、纯读取投影、运行时拦截、持久决策与 exact request 安全恢复；权限拒绝或策略修订产生的无结果终态明确标为 not_attempted，不进入投递恢复；待确认请求按 run 冻结接收目标投影及重放到 Nexus DM/Room/IM Session，没有接收目标时可冻结来源 Session 作为审批兜底。
//   - permission_im_command.go / im_notification.go：只在会话内唯一请求上解析的无 ID `/y`、`/a`、`/d` IM 审批，以及面向 Nexus Session 与外部 IM、带任务身份的控制面通知；普通结果保持原文。
//   - task_support.go：任务容量与 isolated Session 清理；Session 删除时保留任务、停用调度并持久标记待重绑，创建来源只保留 provenance。
//   - summary_heartbeat_tasks.go：heartbeat 汇总。
//   - command_*.go / runtime_command_capability.go：nexus.command MCP 的严格
//     contract/inspect/plan/apply、当前会话查询、跨 Agent/Session 路由、配置/run 双重
//     revision/digest、durable command replay（run receipt 丢失时由 exact run ledger 对账）与只在唯一活跃 physical round 生效的 capability。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
