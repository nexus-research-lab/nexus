// Package tool defines the fixed model-facing Execution Orchestration tool set.
//
// L2 | 父级: internal/mcp/execution（L2 见其 doc.go）
//
// The fixed twelve-tool surface accepts semantic intent only.
// prepare_plan_execution seals one complete strict Plan Document string;
// plan_execution materializes only its proposal id+digest. The document operation
// distinguishes immutable replanning from atomic transient objective replacement;
// abandon_execution cancels a transient graph without a successor.
// Adapters reload authoritative state, inject revisions and idempotency keys,
// clear stale WorkBinding context after a successor/abandon transition, and
// never expose Attempt bookkeeping such as start_work.
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package tool
