// Package orchestration 持久化 Execution、WorkGraph 与 Runtime Graph 关系模型。
// Execution 生命周期见 docs/specs/execution-orchestration-spec.md；Runtime Graph
// 与只读投影见 docs/specs/execution-graph-spec.md。本头部只列存储边界。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//   - repository.go / commands.go / errors.go：事务、command 幂等、CAS 与稳定错误。
//   - execution*.go / goal_retarget.go / evidence.go：Execution 创建、转换、Goal
//     predecessor/successor binding（含不改写既有 terminal predecessor 的幂等 successor reservation）、持久证据与 current/history 查询。
//   - plan.go / plan_proposal*.go：immutable Plan 与 non-authoritative sealed proposal、
//     materialization lease/receipt/recovery。
//   - goal_confirmation.go：与 Goal-bound Execution mutation 同事务的 pending receipt、
//     confirmed CAS 与跨进程扫描。
//   - assignment.go / attempt.go / submission.go / state.go：责任、执行、交付、验收
//     和 Block/Resume/Takeover。
//   - dispatch.go / review_dispatch.go / cancellation_dispatch.go：work、review 与
//     physical cancellation outbox。
//   - subagent_reconciliation.go：child Attempt 与 parent-exit deadline 对账。
//   - runtime_graph*.go：Agent/Tool/Subagent/Gate NodeRun、EdgeRun 与 Artifact ref。
//   - query.go / scan.go / workgraph.go：Snapshot SQL 投影和 managed WorkGraph 读取。
//
// 主要暴露接口：NewRepository/NewSQLRepository 与 Repository 的事务 command、
// owner/session/Goal 查询、GetSnapshot、outbox claim/deliver/retry、Runtime Graph
// upsert/read，以及 Plan proposal 和 Goal confirmation recovery receipt 方法。
package orchestration
