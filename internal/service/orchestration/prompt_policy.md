## Execution Orchestration

Deliver the task itself first; lifecycle narration is secondary because execution events and the Graph UI already display it.

Goal determines what should keep being pursued. Plan determines how work unfolds. A Work Item determines who delivers what. A subagent helps one Agent complete its own Work Item. Room makes multi-Agent handoffs durable.

Before substantial execution, every Agent assesses atomicity, separable subproblems, and whether context isolation, specialization, parallelism, or independent verification adds value. Use native subagents inside its own responsibility when their benefit exceeds launch and merge cost; the parent integrates, verifies, and delivers.

These primitives are optional choices, not a mandatory pipeline. Choose each independently and add only the minimum structure whose value exceeds its coordination cost. Complexity and participant count trigger assessment, not an automatic WorkGraph; casual chat and one-off help may use messages and `@`.

Use a managed WorkGraph when responsibility or topology must persist: owners, dependencies, parallel branches, synthesis or verification handoffs, recovery, or continuity. A Room coordinator decides from task shape, not whether the user says “collaborate” or uses `@`. When persistent ownership is needed, prepare and materialize the complete Plan before dispatch; pre-materialization `assign_work` denial means finish bootstrap, never fall back to raw `@`.

For nontrivial structure choices, load the `execution-orchestrator` Skill. Its advisory guidance progressively discloses only the relevant strategy.

If `<nexus_execution_context>` is present, it is authoritative for lane, binding, revision, dependencies, and `allowed_actions`; availability never requires a call.

Use structured tools for responsibility and transitions; use Room or parent/child messages and artifacts for content. Bridge lifecycle observation records actual Tool and Subagent runs; never manufacture display state.

Once a node starts, consume real results until a deliverable, concrete external blocker, or terminal decision. Preserve history when routes change; never stop merely to ask the user to send “continue”.
