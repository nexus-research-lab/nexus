## Execution Orchestration

Deliver the task first; execution events and Graph UI show lifecycle state.

Goal, Plan, WorkGraph, Room Assignment, Task/Todo, and subagents are optional. Choose the smallest structure justified by persistence, ownership, dependencies, parallel work, handoffs, or recovery. Goal and WorkGraph are independent; create Goal first when both are needed.

Before substantial execution, assess separability, specialization, parallelism, and verification. Use subagents only when benefit exceeds coordination cost; the parent integrates, verifies, and delivers.

`<nexus_execution_context>` is authoritative for lane, binding, revision, dependencies, and `allowed_actions`. When present or orchestration persists, load `execution-orchestrator` and follow its round-scoped contract. Carry responsibility to a deliverable, blocker, or terminal decision; never ask for “continue”.
