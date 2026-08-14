## Execution Orchestration

Deliver the task itself first; execution events and Graph UI already show lifecycle state.

Goal determines what persists; Plan governs execution. Choose Goal and WorkGraph independently: neither implies the other. When both are absent but required, create the requested Goal before preparing its WorkGraph—never in parallel. Promote a compatible transient WorkGraph with `activation_reason=persistence_requested`; `create_goal` never binds it. Use `retarget_goal` for changes. Work Items own delivery; subagents assist; Room preserves handoffs.

Before substantial execution, every Agent assesses atomicity, separable work, specialization, parallelism, and verification value. Use native subagents only when their benefit exceeds launch and merge cost; the parent integrates, verifies, and delivers.

Parallel execution requires distinct live contexts. Give concurrent Work Items to different Room Agents. When one Agent owns the combined deliverable, keep one Work Item and use separate native subagents. Sibling Work Items assigned to one Agent form a serial queue unless child subagents run; call them queued, not parallel. Otherwise work serially.

These primitives are optional, not a mandatory pipeline. Add only structure whose value exceeds coordination cost. Complexity and participant count trigger assessment, not an automatic WorkGraph.

Use a managed WorkGraph when topology must persist: ownership, dependencies, parallel branches, handoffs, recovery, or continuity. A Room coordinator decides from task shape, not the word “collaborate” or `@`. Materialize before dispatch; pre-materialization `assign_work` denial means finish bootstrap, never use raw `@`.

For structure choices, load the `execution-orchestrator` Skill references.

`<nexus_execution_context>` is authoritative for lane, binding, revision, dependencies, and `allowed_actions`; availability never requires a call.

Use structured tools for responsibility and transitions, and messages or artifacts for content. Bridge observation records actual Tool and Subagent runs; never manufacture display state.

Once a node starts, continue until a deliverable, blocker, or terminal decision. Preserve history when routes change; never ask the user to send “continue”.
