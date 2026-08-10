# Scheduled Automation Permission Pipeline

Scheduled automation runs without an attached user turn, so a transient session prompt cannot be its authorization source. Nexus owns the durable authorization decision; sessions only carry the execution route and history.

## Authority boundaries

- The current Agent `AllowedTools` list seeds grants when a task is created. A grant created from this snapshot remains valid only while the current Agent still allows the tool.
- The current Agent `DisallowedTools` list is a hard deny. A task-level approval cannot override it.
- Explicit owner approval can grant one exact run/input or the current task capability. Task grants may include connector, effect, and resource scope.
- Connector OAuth readiness is checked independently from capability authorization. A valid task grant does not imply that a token is still usable.
- Session keys and round IDs are routing and audit fields. They never prove authorization.

## Durable model

Each task owns a versioned `TaskPermissionPolicy` and one permission state. Each run records the policy revision it started with, its block state, the request that blocked it, and whether an external or workspace side effect has started. Each user interaction is an owner-scoped `AutomationPermissionRequest`; the client submits the exact job, run, request, and policy revision it rendered, and storage resolves it only while that request is still the task's current interaction and the run is still blocked by it.

Task definition changes that affect Agent, instruction, execution kind, or session target advance the policy revision. Pending requests are superseded and blocked runs from the older revision are cancelled, so an approval cannot cross a changed execution boundary.

## Runtime flow

1. A run snapshots the current task policy revision.
2. A tool request is normalized into a capability: runtime tool name, connector, effect, resource scope, and exact input fingerprint.
3. Agent hard-deny is checked first, then task grants, then an approved one-run grant.
4. Missing authorization persists a request, moves the task and run into a blocked state, and releases the scheduler runtime claim without incrementing the failure streak. While the exact physical attempt is being interrupted, every later tool request from that attempt is denied, including tools that were otherwise granted.
5. `allow_once` grants the exact input fingerprint on the same logical run. `allow_task` writes a scoped grant and advances the task policy revision. `deny` terminates the run.
6. Connector-backed tools check the active connection after authorization. A missing connection produces a separate reauthorization request.
7. Non-read tools mark `effect_started` before they execute. Approval auto-resumes only while no effect has started; otherwise the run moves to `ready_to_retry` and requires explicit owner confirmation.

Every resume keeps the same `run_id` and starts a new attempt. The resumed instruction names the blocked tool and invalidates the previous denial as a conclusion. A resumed attempt may finish as succeeded only after its task permission handler observes that exact tool being requested and allowed; a textual success without the retry is recorded as failed. This makes retries auditable without pretending they were independent scheduled triggers.

## Main Session alignment

Main Session tasks persist `job_id`, `run_id`, owner, and policy revision in the host-owned system event. Permission resumes additionally carry the resolved request ID, so heartbeat can rebuild the exact retry instruction and verification boundary. Heartbeat consumes task-bound events one at a time and dispatches the original task with its own permission handler. Generic heartbeat work keeps the legacy Agent-default handler. Approval resumes a Main Session run by re-enqueuing the same logical run rather than dispatching an anonymous heartbeat instruction.

## Scripts and compatibility

Script permission is bound to an exact content hash plus owner and Agent. Direct user creation or editing confirms that exact script; Agent-created scripts require approval. Editing script content invalidates the previous grant.

Existing tasks are lazily backfilled from their current Agent defaults. Existing script tasks receive a hash-bound compatibility grant, preserving behavior after migration. New permissions therefore do not silently disable previously working automations.

## User interaction API

- `GET /capability/scheduled/permission-requests?status=actionable`
- `POST /capability/scheduled/permission-requests/{request_id}/decision`
- `POST /capability/scheduled/tasks/{job_id}/runs/{run_id}/permission/resume`

The scheduled-task board attaches only a request whose run is still blocked or explicitly ready to retry. It renders tool approval, connector navigation/recheck, task-input editing, denial, or explicit retry. Permission state remains the primary attention reason; a provider or delivery failure is shown only as an attached diagnostic. The controller owns all API calls and commits authoritative task results before background refresh.
