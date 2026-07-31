// Package orchestration 是 Goal 可选绑定下的 Execution、Plan、Work Item、Assignment、
// Attempt、Submission 与 Acceptance 业务状态机。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / errors.go / work_binding.go / coordination_round.go：领域装配、输入校验、身份、structured Room WorkBinding/ReviewBinding、review-to-coordination 升级、exact Goal 与 round-scoped Coordination capability fence、optimistic revision 错误。
//   - execution.go / execution_transition.go / goal_retarget.go / plan_validation.go：transient Execution、Goal revision predecessor supersede、首次 Execution+Plan 原子创建、objective replacement/abandonment、immutable Plan draft、DAG 与 output scope 校验。
//   - commands.go：责任归属、真实执行尝试、Submission/Acceptance、Block/Resume、统一投影集合上限、同一 Execution 内显式 Plan revision replacement 与接管。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go / room_attempt_terminal.go：Assignment target preflight、带 current Spec/accepted dependency WorkContract 的 Room Assignment outbox、permanent cancel/transient retry、review-return 与 exact runtime cancellation outbox consumer port、review admission 与 slot root Attempt 终态桥。
//   - subagent_admission.go：SDK Agent tool 强准入、后端 launch binding、child Attempt 生命周期、parent round exit durable reconciliation 与 terminal evidence。
//   - review.go：Submission、Acceptance、依赖解锁与 completion audit。
//   - context.go / execution_view.go：面向当前 actor 的有界 <nexus_execution_context>、确定性 graph digest、current input/output、已验收依赖、terminal subagent result ref、恢复/guarded replan、objective transition affordance，以及去除 capability identity 的 Web WorkGraph 只读投影。
//   - prompt.go / prompt_policy.md：所有 DM、Room、Goal continuation 共用的稳定执行契约。
//   - goal_policy.go / promotion.go / evidence.go / explicit_goal.go：fail-closed adaptive promotion、runtime/scheduler 持久证据，以及 explicit/adaptive Goal revision successor 的双向 binding/confirmation。
//   - result.go：所有模型可见 mutation 共用的 outcome/reason/next_actions envelope。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/doc.go（L2）与 AGENTS.md（L1）
package orchestration
