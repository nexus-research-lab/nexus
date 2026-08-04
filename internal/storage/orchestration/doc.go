// Package orchestration 持久化 Execution Orchestration 的关系模型。
//
// L2:
//   - repository.go: Repository、command 幂等、Execution CAS 与事件序列事务。
//   - errors.go: 稳定存储错误与可安全重开事务的瞬时冲突分类。
//   - commands.go: 有语义的存储命令。
//   - execution.go / execution_transition.go / goal_retarget.go / evidence.go: Execution 创建、Goal revision graph supersede 与 reserved successor 校验、首次 Plan 原子写入、transient replacement/abandonment、查询、Goal 绑定、runtime/scheduler 持久证据与完成。
//   - plan.go: immutable Plan/Spec/membership/dependency/output claim 写入、执行契约集合上限防御与显式 active-work replacement。
//   - assignment.go: Assignment、Dispatch、root Attempt 与 takeover。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go: Room Assignment、跨 Agent Submission review-return 与 exact runtime cancellation outbox 的 list/claim/deliver/retry/cancel/recovery lease CAS；自审不制造回投；永久 Assignment Dispatch 失败只在 current active Plan/Spec 下同事务回收尚未启动的责任，stale graph 只终结旧 outbox；所有主动收束 live Attempt 的控制路径在同事务、状态更新前统一捕获 target。
//   - attempt.go / subagent_reconciliation.go: root-only Room round identity、同 round 多 child tool binding、Attempt start/terminal 生命周期，以及精确 T+30s parent round exit grace deadline 的 durable schedule/跨进程 expired 查询。
//   - runtime_graph.go: Bridge 生命周期事件及可选 alignment Gate 形成的 provider-neutral Agent/Tool/Subagent/Gate NodeRun 与运行边幂等存储；与 command CAS 隔离。
//   - submission.go: immutable Submission、按需同事务跨 Agent review-return outbox 与 append-only Acceptance。
//   - state.go: 显式 Block/Resume、旧 Assignment/Attempt/Dispatch 收束与派生 readiness/completion。
//   - query.go / scan.go: 有界 Snapshot 与 SQL row 投影。
package orchestration
