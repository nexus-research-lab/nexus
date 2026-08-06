// Package orchestration 是 Goal 可选绑定下的 Execution、Plan、Work Item、Assignment、
// Attempt、Submission 与 Acceptance 业务状态机。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / errors.go / work_binding.go / coordination_round.go：领域装配、输入校验、身份、structured Room WorkBinding/ReviewBinding、review-to-coordination 升级、exact Goal 与 round-scoped Coordination capability fence、optimistic revision 错误。
//   - execution.go / execution_transition.go / goal_retarget.go / plan_validation.go：transient Execution、Goal revision predecessor supersede、首次 Execution+Plan 原子创建、已有 planless Execution 的首个 Plan、objective replacement/abandonment、immutable Plan revision、DAG 与 output scope 校验。
//   - plan_document.go / plan_proposal.go / plan_materialization.go / plan_proposal_recovery.go：strict Nexus Plan Document v1 解析、durable non-authoritative sealed proposal、owner/session/scope/coordinator/target 以及 Goal owner/activation/reserved successor/predecessor digest fence、ambient Goal 重新校验、authoritative command receipt 归因、不可继承的 initial lease 与 materializer CAS claim、receipt-proven blocked race 自动收敛、幂等原子 materialization，以及缺少 confirmer 时仍 fail closed 的 Goal confirmation pending saga 与跨 round/process reconciler。
//   - commands.go：责任归属、真实执行尝试、Submission/Acceptance、Block/Resume、统一投影集合上限、同一 Execution 内显式 Plan revision replacement 与接管。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go / room_attempt_terminal.go：Assignment target preflight、带 current Spec/accepted dependency WorkContract 的 Room Assignment outbox、permanent cancel/transient retry、跨 Agent review-return 与 exact runtime cancellation outbox consumer port、review admission 与 slot root Attempt 终态桥；自审沿 WorkBinding 同轮继续。
//   - subagent_admission.go：SDK Agent tool 的 runtime-only 放行与可选 managed launch binding、瞬时存储冲突退避重试、child Attempt 生命周期、parent round exit durable reconciliation 与 terminal evidence。
//   - review.go：Submission、Acceptance、依赖解锁与 completion audit。
//   - context.go / execution_view.go / runtime_graph*.go / execution_alignment.go：面向当前 actor 的有界 <nexus_execution_context>、确定性 graph digest、current input/output、已验收依赖、terminal subagent result ref、只反馈当前 Agent observed runtime facts 的恢复上下文、guarded replan、Agent-selected objective alignment Gate，以及在不扩张运行 Snapshot 的前提下为 WorkGraph 读取完整 child Attempt 历史、去除 capability identity、保留每个 sibling Subagent、从结构化 Agent task/result 或受限历史 transcript 恢复 exact Subagent 与 child Tool lifecycle、让 launch Tool 只作关联证据而不覆盖 child 状态、过滤无独立运行身份的 progress facet、按 exact launch tool 与 durable parent identity 保证子图归属和边完整、按用户可观察动作类别及运行/失败/控制边/Artifact/通用 visibility hint 提升 Tool 可见性、为每个 Subagent 保留有界代表 Tool 槽位并在关键动作后补入最近成功动作、每个 Tool NodeRun 独立且只以 exact retry edge 关联、无关联时仍提升同 owner 同类工具失败后的最后成功节点、以成功 submit_work 精确锚定 review/control return、避免重复领域 mutation 节点并保留 supporting inspection Tool 的 detail 历史与到达顺序无关 exact Tool Artifact、显式 partial/total、最近 managed WorkGraph 不受更新 planless round 覆盖、只记录已发生控制返回与 exact retry 事实的 Web WorkGraph/Runtime Graph 只读投影。
//   - prompt.go / prompt_policy.md：所有 DM、Room、Goal continuation 共用的稳定执行契约。
//   - goal_policy.go / promotion.go / evidence.go / explicit_goal.go：只受权限与状态硬门槛约束的 Agent-selected adaptive promotion、runtime/scheduler 建议信号，以及 explicit/adaptive Goal revision successor 的双向 binding/confirmation。
//   - result.go：所有模型可见 mutation 共用的 outcome/reason/next_actions envelope。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 internal/protocol/doc.go（L2）与 AGENTS.md（L1）
package orchestration
