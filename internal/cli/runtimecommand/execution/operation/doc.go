// Package operation defines the model-facing Execution Orchestration and
// WorkGraph Workflow distillation command set.
//
// L2 | 父级: internal/cli/runtimecommand/execution（L2 见其 doc.go）
//
// The fixed operation surface accepts semantic intent only.
// prepare_plan_execution seals one complete strict Plan Document string plus a
// none/current/inherit Goal binding intent; current consumes only exact round authority;
// plan_execution materializes only its proposal id+digest. The document operation
// distinguishes immutable replanning from atomic transient objective replacement;
// abandon_execution cancels a transient graph without a successor.
// Adapters reload authoritative state, inject revisions and idempotency keys,
// keep malformed Plan transport retry state across per-command registry rebuilds,
// preserve exact bound Room work/review reads instead of downgrading them to
// observation, clear stale WorkBinding context after a successor/abandon
// transition, and never expose Attempt bookkeeping such as start_work. Command
// results may compact optional runtime facts and graph digests, but never erase
// responsibility context or fabricate a physical-round refresh. A committed
// Execution -> Goal mutation whose reverse confirmation is still recovering is
// returned as applied/noop with goal_confirmation_status=pending and an
// executable retry next_action, never as a transport IsError.
// distill_workgraph_workflow is the only model write boundary for reusable
// Workflow commands: it consumes exact historical Work Item locators and
// persists semantic nodes/dependencies without runtime or delivery history.
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package operation
