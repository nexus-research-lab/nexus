// Package operation defines the model-facing Execution Orchestration and
// Named WorkGraph save command set.
//
// L2 | 父级: internal/mcp/command/execution（L2 见其 doc.go）
//
// The fixed operation surface accepts semantic intent only.
// prepare_plan_execution seals one complete strict Plan Document string plus a
// none/current/inherit Goal binding intent; current consumes only exact round authority;
// plan_execution uses empty model input and materializes only the host-owned durable
// proposal binding; optional legacy id+digest can only match that binding. The document operation
// distinguishes immutable replanning from atomic transient objective replacement;
// abandon_execution cancels a transient graph without a successor.
// Adapters reload authoritative state, inject revisions and idempotency keys,
// keep malformed Plan transport retry state across per-command registry rebuilds,
// preserve exact bound Room work/review reads instead of downgrading them to
// observation, clear stale WorkBinding context after a successor/abandon
// transition, and never expose Attempt bookkeeping such as start_work. Command
// mutation results omit optional runtime facts and may compact graph digests,
// while explicit inspect returns bounded runtime history; neither may erase
// responsibility context or fabricate a physical-round refresh. A committed
// Execution -> Goal mutation whose reverse confirmation is still recovering is
// returned as applied/noop with goal_confirmation_status=pending and an
// executable retry next_action, never as a transport IsError.
// WorkGraph authoring is one service with two constrained registries. Ordinary
// DM/Room rounds can inspect exact-session sources and Drafts, extract/reuse one
// completed graph, append a full CAS revision, select an immutable version, and
// save only after explicit user confirmation. Hidden Nexus-main-Agent editor
// Sessions expose only revise/select for their bound Draft. UI confirmation uses
// an isolated internal registry where distill_workgraph consumes only the exact
// host-bound preview_id and persists that selected sketch unchanged, without
// source transcript, runtime or delivery history.
//
// 成员清单：
//   - registry.go / schema.go / adapter.go：operation 注册、严格输入协议与共用可靠性适配。
//   - plan.go / work.go：Plan 两阶段提交与 Assignment/Submission/Review 生命周期。
//   - goal.go / workflow.go：Execution-Goal 边界与 WorkGraph authoring/edit/save。
//   - read.go / abandon.go：权威读取与无 successor 的 transient Execution 放弃。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package operation
