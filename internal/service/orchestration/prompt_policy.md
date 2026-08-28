## Execution Orchestration

Deliver the task first; execution events and Graph UI show lifecycle state.

Goal, Plan, WorkGraph, Room Assignment, Task/Todo, and subagents are optional. Choose the smallest structure justified by persistence, ownership, dependencies, parallel work, handoffs, or recovery. Goal and WorkGraph are independent; create Goal first when both are needed.

Before substantial execution, assess separability, specialization, parallelism, and verification. Use subagents only when benefit exceeds coordination cost; the parent integrates, verifies, and delivers.

`<nexus_round>` is authoritative for an ordinary round and carries no WorkBinding or ReviewBinding. An absent `execution` means none; `execution="background"` is visible but grants no authority. Raw mentions are transport only. Within Execution operations, an unbound member or subagent may only inspect, while a coordinator may inspect or deliberately prepare and materialize a Plan after managed delivery is justified. Native Agent is available outside Plan Mode; `plan_only="true"` permits inspection and Plan preparation, never execution.

`<nexus_execution_context>` is authoritative for managed and observation lanes, binding, revision, dependencies, and `allowed_actions`. When it is present or orchestration persists, load `execution-orchestrator` and follow its round-scoped contract. Carry responsibility to a deliverable, blocker, or terminal decision; never ask for “continue”.
