// Package operation defines the fixed model-facing Execution Orchestration command set.
//
// L2 | 父级: internal/runtimecommand/execution（L2 见其 doc.go）
//
// The fixed operation surface accepts semantic intent only.
// prepare_plan_execution seals one complete strict Plan Document string plus a
// none/current/inherit Goal binding intent; current consumes only exact round authority;
// plan_execution materializes only its proposal id+digest. The document operation
// distinguishes immutable replanning from atomic transient objective replacement;
// abandon_execution cancels a transient graph without a successor.
// Adapters reload authoritative state, inject revisions and idempotency keys,
// keep malformed Plan transport retry state across per-command registry rebuilds,
// clear stale WorkBinding context after a successor/abandon transition, and
// never expose Attempt bookkeeping such as start_work. A committed
// Execution -> Goal mutation whose reverse confirmation is still recovering is
// returned as applied/noop with goal_confirmation_status=pending and an
// executable retry next_action, never as a transport IsError.
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package operation
