// Package orchestration 实现 Goal 可选绑定的 Execution 与 WorkGraph 状态机。
// 当前产品语义以 docs/specs/execution-orchestration-spec.md 为准；Runtime Graph
// 与 Web 投影以 docs/specs/execution-graph-spec.md 为准。本头部只保留代码地图。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员地图：
//   - service.go / errors.go / work_binding.go / coordination_round.go：装配、actor、
//     Work/Review/Coordination capability 与 optimistic revision fence。
//   - execution*.go / goal_retarget.go / plan_validation.go：Execution 生命周期、
//     predecessor/successor、Plan revision 与 DAG 校验。
//   - plan_document*.go / plan_proposal*.go / plan_materialization.go：严格 Plan
//     document、非权威 sealed proposal、原子 materialization 与重启恢复。
//   - commands.go：Assignment、Attempt、Submission、Acceptance、
//     Block/Resume/Takeover 与 completion。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go /
//     room_attempt_terminal.go：Room work、review 和 physical cancellation outbox，
//     以及 dispatched/self WorkBinding 共用的 root Attempt 终态桥。
//   - subagent_admission.go：Subagent admission、child Attempt、parent-exit deadline、合并唤醒信号与重启 orphan 对账。
//   - runtime_graph*.go / execution_view.go / context.go / execution_alignment.go：
//     Runtime Graph 事实、actor context、目标对齐与 managed WorkGraph 只读投影。
//   - goal_policy.go / promotion.go / explicit_goal.go / goal_binding.go /
//     goal_confirmation_recovery.go：Goal promotion、双向 binding 五态与 durable
//     confirmation receipt/reconciler。
//   - completion_audit_recovery.go：accepted Review 后的 blocker-aware durable
//     completion reconciler；不替代模型可见 alignment audit。
//   - invalidation.go / result.go：owner/session 只读投影失效 port、宿主消费的
//     responsibility advancement receipt 与稳定 mutation outcome/next-action envelope。
//   - prompt.go / prompt_policy.md：DM、Room 与 Goal continuation 共用执行提示。
//
// 主要暴露接口：NewService 与 Service 的 Ensure/Get*/RuntimeContext、Plan、work、
// review、Goal binding/promotion、Runtime Graph observation 和 recovery 方法；
// Set*Gateway/Consumer/Sink 只注入 Goal、Room、runtime 与 transport 的消费侧 port。
//
// [PROTOCOL]: 行为或 wire 变化时检查两份 execution spec、protocol L2 与 AGENTS.md。
package orchestration
