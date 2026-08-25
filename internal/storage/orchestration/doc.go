// Package orchestration 持久化 Execution、WorkGraph 与 Runtime Graph 关系模型。
// Execution 生命周期见 docs/specs/execution-orchestration-spec.md；Runtime Graph
// 与只读投影见 docs/specs/execution-graph-spec.md。本头部只列存储边界。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//   - repository.go / commands.go / errors.go：事务、command 幂等、CAS 与稳定错误。
//   - execution*.go / goal_retarget.go / evidence.go：Execution 创建、转换、Goal
//     predecessor/successor binding（含不改写既有 terminal predecessor 的幂等 successor reservation）、持久证据与 owner/session managed history 查询。
//   - plan.go / plan_proposal*.go：immutable Plan、non-authoritative sealed proposal、
//     exact owner/session/scope active binding 与 materialization lease/receipt/recovery。
//   - goal_confirmation.go：与 Goal-bound Execution mutation 同事务的 pending receipt、
//     confirmed CAS 与跨进程扫描。
//   - completion_audit.go：与 accepted Review 同事务的 completion receipt、
//     Complete 同事务终结、CAS defer/discard 与跨进程扫描。
//   - assignment.go / attempt.go / submission.go / state.go：责任、执行、交付、验收
//     和 Block/Resume/Takeover。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go：work、review 与
//     physical cancellation outbox。
//   - background_deadline.go：三类 outbox、Subagent reconciliation 与三类 saga 的
//     最早 durable deadline 聚合读面；只负责 timer 索引，不 claim 或改写业务状态。
//   - subagent_reconciliation.go：child Attempt 的 parent-exit deadline 与上次进程未落 deadline orphan 对账。
//   - runtime_graph*.go：Agent/Tool/Subagent/Gate NodeRun、EdgeRun 与 Artifact ref。
//   - query.go / scan.go / workgraph.go：Snapshot SQL 投影、跨同一 Execution 的 Plan revision 且与 Snapshot 同一 read transaction 的 append-only Assignment/Attempt/Submission/Review/Acceptance 画布历史，以及 managed WorkGraph 读取。
//
// 主要暴露接口：NewRepository/NewSQLRepository 与 Repository 的事务 command、
// owner/session/Goal 查询、GetSnapshot/GetWorkGraphState、outbox claim/deliver/retry、Runtime Graph
// upsert/read，以及 Plan proposal binding/materialization、Goal confirmation 和 completion audit recovery
// receipt 方法，以及 background deadline snapshots。
package orchestration
