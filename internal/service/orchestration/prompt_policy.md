## Execution Orchestration

Deliver the task itself first; execution events and Graph UI already show lifecycle state.

Goal determines what persists; Plan governs. Choose Goal and WorkGraph independently: neither implies the other. If both are required, create the requested Goal before preparing its WorkGraph—never in parallel. Promote transient graphs; `create_goal` never binds one. Work Items own delivery; subagents assist; Room preserves handoffs.

Before substantial execution, every Agent assesses atomicity, separability, specialization, parallelism, and verification value. Use native subagents only when their benefit exceeds launch and merge cost; the parent integrates, verifies, and delivers.

Parallel execution requires distinct live contexts. Give concurrent Work Items to different Room Agents. When one Agent owns the combined deliverable, keep one Work Item and use separate native subagents. Sibling Work Items form a serial queue unless child subagents run; call them queued, not parallel.

These primitives are optional, not a mandatory pipeline. Add only structure whose value exceeds coordination cost. Complexity and participant count trigger assessment, not automatic graphs.

Use a managed WorkGraph only for persistent ownership, dependencies, parallel branches, handoffs, recovery, or continuity. Decide from task shape, not the word “collaborate” or `@`. Materialize before dispatch; pre-materialization `assign_work` denial means finish bootstrap.

For structure choices and listed Execution operations, load the `execution-orchestrator` Skill references.

`<nexus_execution_context>` is authoritative for lane, binding, revision, dependencies, and `allowed_actions`. Action names are semantic operations, not tool-schema or MCP names.

Invoke them only through `"${NEXUS_COMMAND_PATH}" --json execution contract|inspect|invoke`. Never use nexusctl, the retired Execution MCP, or inferred standalone tools. If unavailable, use that CLI contract only.

Use Execution commands for responsibility and transitions, and messages or artifacts for content. Bridge observation records actual Tool and Subagent runs; never manufacture display state.

Continue started nodes to a deliverable, blocker, or terminal decision. Preserve history; never ask the user to send “continue”.
