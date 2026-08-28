# Execution Orchestration Control-Plane Specification

## 1. Purpose and scope

This document describes the control plane that Nexus implements today for Goal and
managed WorkGraph execution. It defines:

- the three supported product modes and the five Goal/Execution binding states;
- the user-visible Goal control command and its continuation boundary;
- the durable Plan and responsibility model;
- the authority carried by coordinator, worker, reviewer, and Subagent rounds;
- the atomic semantics of the 12 Execution operations and 5 Goal operations;
- transaction, idempotency, reconciliation, and terminal-state invariants;
- current limits that callers and UI code must preserve.

It does **not** define the read-only runtime graph projection or its UI layout.
Those are specified in [Execution Graph Specification](./execution-graph-spec.md).
Only currently implemented control-plane behavior is in scope.

### 1.1 Normative truth sources

This specification owns product semantics and cross-component invariants. Exact
fields, enums, parser rules, and `nexus.command` MCP contracts remain code-generated or
code-defined truth:

| Concern | Truth source |
| --- | --- |
| Execution, Plan, Work Item, Assignment, Attempt, Submission, Acceptance, and events | [`internal/protocol/execution.go`](../../internal/protocol/execution.go) |
| Plan proposal lifecycle and receipts | [`internal/protocol/execution_plan_proposal.go`](../../internal/protocol/execution_plan_proposal.go) |
| Execution/Goal confirmation recovery receipts | [`internal/storage/orchestration/goal_confirmation.go`](../../internal/storage/orchestration/goal_confirmation.go), [`internal/service/orchestration/goal_confirmation_recovery.go`](../../internal/service/orchestration/goal_confirmation_recovery.go) |
| Goal and Goal/Execution binding states | [`internal/protocol/goal.go`](../../internal/protocol/goal.go) |
| Durable Goal continuation launch receipts | [`internal/storage/goal/continuation_plan.go`](../../internal/storage/goal/continuation_plan.go), [`internal/service/goal/continuation.go`](../../internal/service/goal/continuation.go) |
| Collection and projection limits | [`internal/protocol/execution_limits.go`](../../internal/protocol/execution_limits.go) |
| Plan Document parsing and normalization | [`internal/service/orchestration/plan_document.go`](../../internal/service/orchestration/plan_document.go) |
| Parser-backed Plan Document contract | [`internal/service/orchestration/plan_document_contract.go`](../../internal/service/orchestration/plan_document_contract.go) |
| Proposal sealing and materialization | [`internal/service/orchestration/plan_proposal.go`](../../internal/service/orchestration/plan_proposal.go), [`plan_materialization.go`](../../internal/service/orchestration/plan_materialization.go) |
| `nexus.command` MCP envelope, operation contract, and typed receipt | [`internal/mcp/command/contract.go`](../../internal/mcp/command/contract.go), [`tool.go`](../../internal/mcp/command/tool.go), [`internal/mcp/round_state.go`](../../internal/mcp/round_state.go) |
| Execution operation inputs, schemas, and directory | [`internal/mcp/command/execution/operation/input.go`](../../internal/mcp/command/execution/operation/input.go), [`schema.go`](../../internal/mcp/command/execution/operation/schema.go), [`registry.go`](../../internal/mcp/command/execution/operation/registry.go) |
| Goal operation directory and contracts | [`internal/mcp/command/goal/operation/registry.go`](../../internal/mcp/command/goal/operation/registry.go), [`internal/mcp/command/goal/contract/contract.go`](../../internal/mcp/command/goal/contract/contract.go) |
| Round-scoped structured command adapter | [`internal/mcp/command/tool.go`](../../internal/mcp/command/tool.go), [`internal/app/server/runtime/command.go`](../../internal/app/server/runtime/command.go) |
| Runtime responsibility and coordination authority | [`internal/runtime/responsibility_authority.go`](../../internal/runtime/responsibility_authority.go), [`internal/service/orchestration/work_binding.go`](../../internal/service/orchestration/work_binding.go), [`coordination_round.go`](../../internal/service/orchestration/coordination_round.go) |
| Accepted-review completion recovery receipts | [`internal/storage/orchestration/completion_audit.go`](../../internal/storage/orchestration/completion_audit.go), [`internal/service/orchestration/completion_audit_recovery.go`](../../internal/service/orchestration/completion_audit_recovery.go) |
| Background dispatch and recovery scheduling | [`internal/infra/duework/loop.go`](../../internal/infra/duework/loop.go), [`internal/service/orchestration/background_coordinator.go`](../../internal/service/orchestration/background_coordinator.go), [`internal/storage/orchestration/background_deadline.go`](../../internal/storage/orchestration/background_deadline.go) |
| Room collaboration attribution and handoff recovery | [`internal/protocol/room.go`](../../internal/protocol/room.go), [`internal/storage/workspace/room_public_handoff.go`](../../internal/storage/workspace/room_public_handoff.go), [`internal/service/room/realtime/public_handoff.go`](../../internal/service/room/realtime/public_handoff.go) |
| Host-owned command receipt classification | [`internal/mcp/receipt.go`](../../internal/mcp/receipt.go), [`internal/service/dm/goal_runtime.go`](../../internal/service/dm/goal_runtime.go), [`internal/service/room/realtime/goal_runtime.go`](../../internal/service/room/realtime/goal_runtime.go) |
| UI/Slash Goal command dispatch and durable control history | [`internal/service/slashcommand/goal.go`](../../internal/service/slashcommand/goal.go), [`internal/service/dm/goal_command.go`](../../internal/service/dm/goal_command.go), [`internal/service/room/realtime/goal_command.go`](../../internal/service/room/realtime/goal_command.go) |

Do not copy the complete Plan Document field list or operation input schema into this
document. When this document and those sources disagree on a field shape or enum,
the source above wins and this document must be corrected.

## 2. Stable product model

### 2.1 Supported modes

Goal and WorkGraph are independently useful. Their integration is explicit rather
than inferred from session proximity.

| Mode | Durable state | Meaning |
| --- | --- | --- |
| Goal-only | Active Goal, no confirmed managed WorkGraph binding | Objective lifecycle, limits, continuation, blocking, and completion operate without a WorkGraph. |
| WorkGraph-only | Execution with an active materialized Plan and non-empty Work Items, no Goal fence | The managed graph executes independently. An ambient Goal in the same session is irrelevant. |
| Goal + WorkGraph | Active Goal and Execution with an exact confirmed bilateral binding | Goal objective/revision fences Plan authoring, retargeting, completion, and continuation. |

Both model `create_goal` and the host Goal command create a `goal_only` Goal.
They never reserve or bootstrap an Execution. A fresh Goal enters managed mode
only when a sealed `goal_binding=current` Plan proposal begins materialization;
that proposal owns the stable Execution identity, writes the Goal-side pending
binding before the authoritative Execution/Plan mutation, and confirms the
bilateral binding afterward. `goal_binding=none` never reads or mutates an
ambient Goal. Historical `reserved` Goals and retarget successors remain
recoverable, but they are compatibility/transition states rather than the
default result of setting a Goal.

An Execution row without an active materialized Plan is a bootstrap or
reconciliation state. It is not a fourth public product mode and must not be
presented as a managed WorkGraph.

### 2.1.1 DM/Room entry convergence

Transport chooses owner, session, conversation, and responsible Agent; it does
not choose a different lifecycle. The eight supported entry paths converge as
follows:

| Surface | Intent | Canonical entry | Resulting mode and authority |
| --- | --- | --- | --- |
| DM conversation | Model calls `create_goal` | Goal service create transaction | Goal-only; the verified DM Agent owns the exact Goal revision. |
| DM Composer | Goal mode or `/goal` | Host Goal control command | Goal-only; the visible control record is a projection, not a model turn. |
| DM conversation/Composer | Goal + WorkGraph | Create/retarget Goal, then seal and materialize `goal_binding=current` | Goal + WorkGraph only after bilateral confirmation; the current physical round receives the confirmed dynamic responsibility receipt. |
| DM conversation | WorkGraph-only | Seal and materialize `goal_binding=none` | WorkGraph-only; any ambient Goal is ignored. |
| Room conversation | Lead calls `create_goal` | Goal service create transaction with host-verified Room identity | Goal-only; only the persisted lead owns the exact Goal revision. |
| Room Composer | Goal mode or `/goal` | Host Goal control command with Room lead validation | Goal-only; the control record is written to the exact current conversation. |
| Room conversation/Composer | Goal + WorkGraph | Create/retarget Goal, then seal and materialize `goal_binding=current` | Goal + WorkGraph only after bilateral confirmation; coordinator, worker, and reviewer capabilities remain exact and distinct. |
| Room conversation | WorkGraph-only | Seal and materialize `goal_binding=none` | WorkGraph-only; Room membership does not grant Work Item authority. |

`create_goal` never guesses that an unbound transient Execution should be
adopted. Existing WorkGraph intent uses the explicit promotion operation, while
new integrated work uses `goal_binding=current`. This keeps all eight paths on
the same Goal, Execution, and responsibility state machines and removes a
cross-domain half-commit from Goal creation.

### 2.1.2 State ownership and projections

Each concern has one authoritative state. A runtime or UI projection may lag or
disappear; it must never be used to manufacture the durable fact again.

| Concern | Authoritative truth | State transition | Derived or recoverable projections |
| --- | --- | --- | --- |
| Goal | `session_goals` plus append-only `goal_events` | Goal repository CAS on ID, owner, status, version, and objective revision | Round command context, continuation strip, title/control history, and UI blocker text |
| Plan/Execution/WorkGraph | Orchestration SQL aggregate: Execution, immutable Plan revision, Work Item state, Assignment, Attempt, Submission, Acceptance, and outbox/receipt rows | One service command re-reads the aggregate and commits one typed mutation | Execution Graph/UI, runtime nodes, Room delivery, and snapshot caches |
| Responsibility capability | Durable responsibility records are business truth; one mutable in-memory `ResponsibilityAuthorityState` is the current physical round's capability projection | A successful typed mutation receipt atomically replaces or clears Goal, Execution, Work, and Review authority for the next call in that round | Runtime command context and operation availability hints; neither authorizes without service revalidation |
| Collaboration handoff | Owner-scoped Room directed-message/public-handoff/InputQueue ledgers with exact source Goal ID and objective revision | Durable schedule, dispatch, target terminal, and source handback advance separate idempotent stages | Wake state, public feed, source continuation defer, and collaboration audit evidence |
| Continuation | `goal_continuation_plans` open receipt plus Goal continuation count/event | `scheduled → claimed → started → settled`, or retry/release/cancel under revision and lease CAS | Runtime running-round registration and UI continuation state |
| Progress | Typed `applied` mutation outcome under the exact current Goal/Execution responsibility; the resulting Goal controller counters/events are durable | Counted mutation clears the no-progress streak; exact handoff defers; terminal empty/failure advances suppression | Parsed SDK tool results are candidate facts only and cannot count by tool success alone |

Cache attribution is observational, not a seventh control state. The usage ledger
stores provider-reported cache token totals beside low-cardinality lane/surface
fingerprints. Those hashes help compare stable versus changing host context but
are neither provider cache keys nor authorization facts.

### 2.2 Goal/Execution binding states

The resolver exposes exactly five states:

A Goal-free WorkGraph has no Goal binding resolution at all. In particular,
`standalone` is a Goal-side resolver outcome; it is not the state of a
WorkGraph-only Execution.

| State | Meaning | Mutation policy |
| --- | --- | --- |
| `standalone` | The Goal has no Execution relationship. | Goal lifecycle is independent of WorkGraph state. |
| `reserved` | The Goal reserves an Execution identity, but the exact graph is not yet confirmed. | Goal lifecycle remains available; the reservation is provenance, not proof of a Plan. |
| `pending` | Binding materialization or confirmation is incomplete. | Bound operations fail closed until reconciliation confirms both sides. |
| `confirmed` | Goal and Execution agree on identity, owner, session, scope, and objective revision. | Integrated Goal + WorkGraph rules apply. |
| `conflict` | The two sides disagree or violate an identity/revision fence. | Bound operations fail closed and require reconciliation or an explicit correction path. |

`reserved`, `pending`, and `confirmed` may appear in persisted binding metadata.
`standalone` and `conflict` are resolver outcomes. The resolver cross-checks Goal
metadata against authoritative Execution storage; callers must not classify a
binding from one side alone.

Only `confirmed` means the managed graph can participate in Goal completion,
retarget, and continuation. A reserved Execution ID does not imply that an
Execution, active Plan, or Work Item exists.

### 2.2.1 Goal continuation control state

Goal lifecycle status and automatic continuation control are separate concerns.
An active Goal remains `active` while its continuation loop is recovering or
suppressed. Every Goal wire projection exposes the server-derived
`continuation_state=inactive|ready|recovering|suspended`; clients must not
reconstruct its threshold from `empty_progress_count`. The count remains a
durable consecutive no-progress streak, not a paused lifecycle status:

- the first empty continuation emits `continuation_recovery_scheduled` and gets
  one recovery turn with an explicit execute-now boundary;
- a second consecutive empty continuation emits `continuation_suppressed` and
  stops automatic continuation until explicit Goal activity or Resume clears the
  streak;
- provider/runtime terminal failure after a round starts stores `last_error` and
  suppresses immediately; a failure before runtime registration stays on the
  durable launch receipt and retries with bounded backoff without changing the
  Goal lifecycle;
- any counted Goal mutation or explicit user activity clears the streak.

Read-only inspection is not fabricated progress. The recovery turn exists so a
legitimate inspect-then-mutate sequence is not stopped between those stages,
while the second empty turn still closes a real loop.

Tool transport success is not a progress receipt. Only an explicit `applied`
Goal mutation, or an `applied` WorkGraph mutation executed under an exact current
Goal binding, counts as continuation progress. Ordinary message sends,
read/list/search operations, Task/Todo bookkeeping, unknown tools, rejected/no-op
mutations, and unbound WorkGraph mutations fail closed. A durable exact
Goal-attributed Room handoff or queue receipt defers the source continuation until
handback; it does not masquerade as mutation progress.

Continuation scheduling is one durable transaction: the Goal continuation count
and audit event advance together with a server-only `goal_continuation_plans`
receipt containing the exact Goal revision, Execution identity, previous round,
purpose, prompt, and metadata. Prompt content is never copied into Goal metadata
or client projections. A worker obtains a leased CAS claim, registers the exact
runtime round, and only then advances the receipt to leased `started`. `started`
remains the recovery owner and the one-open-plan uniqueness fence until the exact
runtime terminal callback advances it to `settled`; it is not a terminal launch
receipt. Room execution keeps two non-interchangeable identities through that
callback: the outer/root continuation round settles the durable receipt, while
the slot `AgentRoundID` identifies progress, failure, and completion-miss audit
events. A public handoff settles the source root receipt before waiting for the
separate durable handback; the handback may then schedule a fresh continuation
without waiting for lease recovery. A process crash before claim leaves the scheduled receipt recoverable;
a crash after claim or after runtime registration makes the same receipt
recoverable after its lease expires, without incrementing the Goal again.
Duplicate workers cannot claim the same receipt. Objective revision changes and
non-active lifecycle transitions cancel old open receipts. Historical opaque
reservation IDs are discarded because they lack enough authority and prompt
data to replay safely. Migration refunds only distinct, non-empty outstanding
reservation IDs from `continuation_count`, clamped at zero; counts for rounds
that actually ran remain part of the same objective revision's usage limit.
The recovery controller reconciles once at startup, wakes after local Goal or
runtime receipt mutations, arms the exact next retry/lease deadline, and retains
a bounded low-frequency audit for lost hints and cross-process writes.

Every SQL mutation that first makes an Execution Goal-bound writes one exact
`execution_goal_confirmations` pending receipt in the same transaction. The
receipt stores Execution ID, Goal ID, objective revision, and completion criteria;
therefore restart recovery does not depend on the originating request or on a
Plan proposal row. Goal confirmation is idempotent, and the background reconciler
marks the receipt `confirmed` only after the Goal-side reverse binding succeeds.
The Plan proposal confirmation state remains its materialization-saga projection,
not the sole recovery source. Goal confirmation, Plan proposal recovery, and
completion audit share one deadline snapshot/driver while retaining independent
durable state machines and CAS transitions.

### 2.3 User Goal control command

Composer Goal mode and the textual `/goal <objective>` command are two transports
for one Nexus host command. The command is intercepted before runtime input and
must not be interpreted as an ordinary model prompt. It performs these ordered
stages:

1. validate the authenticated DM or Room scope and, for Room, one current member
   as Goal lead;
2. best-effort normalize the objective inside a bounded portion of the request
   ACK window, then create or explicitly replace the current Goal under the
   current-Goal uniqueness, owner, objective-revision, and binding fences;
3. append one user-visible, terminal control record whose canonical content is
   `/goal <objective>` and whose subtype is `goal_set`;
4. attempt to send the durable/transient `chat_ack` and terminal
   `round_status=finished` for that host control round; and
5. dispatch the active Goal continuation through the normal Goal state machine.

The WebSocket ingress completes cheap authenticated owner/session validation
before acceptance. An accepted `set_goal` then keeps its original connection FIFO
position but runs in a bounded detached context that retains the authenticated
context values. Closing the page, switching Session, or losing the socket may lose
the response projection, but cannot cancel an already accepted Goal mutation,
control-record append, directory invalidation, or continuation attempt. The
business deadline remains authoritative: a separate short-lived delivery context
may attempt the terminal ACK or correlated error after that deadline, but cannot
restart or extend the mutation. This transport detachment is not a durable command
receipt and does not by itself make an in-flight command replayable after a server
process crash.

The Web client holds the exact original Session binding and physical shared socket
under the locally minted `client_request_id` until a raw ACK/error, an explicit
destructive Session reset, or the bounded acceptance timeout. A timeout is
`unknown`, not success or rejection. Reconciliation therefore uses the original
`client_message_id` control record, or a newer owner-scoped Goal identity/version
whose `objective` or server-recorded `source_objective` matches the submission;
the currently visible route and an unchanged same-objective Goal are not evidence.

The control record is not a runtime turn and never waits for an assistant/result
terminal. It is still a real visible user record: it starts a draft conversation,
increments the session message count, and gives a new Goal-only session enough
durable state to receive an immediate fallback title. Goal title generation and
Goal continuation therefore do not depend on a preceding ordinary user prompt or
the first model response. For a standalone workspace session, that session is the
title target; for a Room-backed WebSocket DM, its canonical SQL conversation is
the single title target. The current canonical objective is the synchronous
fallback and also drives the normal concise-title generator.
Existing user-defined titles remain immutable. At process startup, current Goals
with durable owner provenance replay this projection once, repairing a missing or
fallback title after a crash or lost response without guessing a legacy owner.

For Room-backed DM sessions, the SQL row owns Room identity, title, runtime settings,
and draft state, while the Agent workspace session owns runtime message progress,
last activity, context usage, and transcript lineage. The unified Session view
merges those fields monotonically; it must not let the legacy SQL `messages` count
replace a newer workspace snapshot. Group conversation `message_count` is rebuilt
from the canonical shared Room ledger and cached by ledger file version. Existing
rows in the legacy `messages` table remain a compatibility lower bound for imported
data, not the live Room history truth source. Goal fallback and generated titles for
a Room-backed DM therefore update the SQL conversation only; they must not attempt
an unsupported workspace Session title mutation.

Continuation is scheduled only after the first control response send has been
attempted; socket delivery success is not a continuation prerequisite. When the
model invokes `create_goal` inside an already-running visible round, its
hidden continuation is suppressed until that round has fully left both runtime
and Goal usage accounting; a terminal UI event alone is not proof that accounting
cleanup has finished.

A Room Goal continuation may request a conversation-only contribution through a
public `@member` or a directed-message wake. The host attaches the exact Goal ID
and objective revision as collaboration attribution across the directed-message
fact, handoff ledger, InputQueue, and restart recovery. Goal-directed wakes write
their deterministic handoff edge before immediate or delayed dispatch; startup
also scans attributed directed-message facts and repairs an interrupted
message-to-handoff write only when that exact Goal revision is still active.
The Room tool call also carries a host-generated idempotency identity derived
from the SDK tool-use identity, with a source-round/canonical-input fallback.
Retries therefore reuse the same directed-message and wake identity; immediate
and delayed wakes are both scheduled durably before queue admission, retry
online while pending, and remain complete after a late tool retry.
This attribution is scheduling provenance, not
`GoalAuthorityState`: the target round remains unable to call Goal mutation tools.
The source continuation is not classified as empty while that attributed handoff
is pending. When the target reaches a terminal state, substantive output resets
the continuation run, public substantive output may be retained as Room-visible
collaboration audit evidence, and the host schedules a fresh authorized lead
continuation. This evidence never gates Goal completion. No-reply, error,
interruption, stale-revision output, and private output never manufacture public
collaboration evidence.

## 3. Durable aggregate

### 3.1 Execution and Plan proposal

An **Execution** is the durable container for one managed orchestration history.
Its status is authoritative and terminal transitions fence late work.

A **Plan proposal** is a sealed, durable, non-authoritative authoring receipt. Its
implemented lifecycle is `sealed`, `materializing`, `materialized`, `blocked`, or
`discarded`. A proposal cannot authorize work by itself.

Only `plan_execution` can materialize the exact sealed proposal into authoritative
Execution/Plan state.

### 3.2 Plan revision

A **Plan** is an immutable revision. Exactly one revision may be active for an
Execution. Replan writes a new revision rather than mutating the old revision.
Superseded and cancelled revisions remain historical facts.

An active Plan contains revision-specific memberships. Each membership identifies:

- a Work Item;
- its parent relationship;
- whether it is required and/or terminal;
- its stable position in the revision.

The exact membership fields are defined in protocol, not duplicated here.

### 3.3 Work Item and delivery contract

A **Work Item** is the durable identity of a business responsibility. Its immutable
specification carries the work kind, subject, objective, acceptance contract,
delivery expectations, assignment policy, and other parser-validated business
fields.

Implemented Work Item kinds are `produce`, `review`, `verify`, and `integrate`.
Those kinds describe business responsibility; they are not runtime graph node
types.

Dependencies are revision-scoped:

- a hard dependency is satisfied only by an accepted upstream Submission;
- a soft dependency is visible context but does not gate readiness.

Output claims are either exclusive or shared. Exclusive claims prevent two active
responsibility chains from owning the same declared output.

Every output scope uses one canonical form: `file:<workspace-relative-path>`,
`dir:<workspace-relative-path>`, or `semantic:<stable-key>`. File and directory
paths use forward slashes, are never absolute, never equal `.`, and never escape
with `..`; an owner/Agent absolute workspace path is not a Plan scope.

These claims are a durable scheduling and review contract, not a filesystem
capability. Nexus validates Plan topology and exposes the assigned scopes to the
responsible runtime, but the current workspace layer does not intercept every
file write against them. `exclusive` therefore means the orchestrator will not
legitimately schedule conflicting responsibility lanes; it must not be described
to users or models as an OS-enforced write lock. Plans that mutate shared
workspace outputs should declare scopes so the scheduler can protect them.

An all-hard dependency path makes overlapping exclusive claims an ordered
ownership handoff rather than concurrent ownership: every downstream Work Item
remains locked until the upstream Submission is accepted, so a later draft,
integration, or finalization step may truthfully declare the same file, directory,
or semantic output. Unrelated branches, siblings, parent nesting, and soft-only
dependency paths do not provide that gate and still conflict unless every
overlapping claim is explicitly shared.

### 3.4 Mutable Work Item state and derived lifecycle

The stored mutable Work Item state is limited to:

- `open`;
- `waiting_input`;
- `cancelled`;
- `superseded`.

`ready`, `assigned`, `running`, `submitted`, and `accepted` are derived from the
active Plan, dependencies, and current responsibility records. They must not be
persisted as a second lifecycle that can drift from Assignment, Attempt,
Submission, or Acceptance history.

A Work Item is ready only when it belongs to the active Plan, remains open, all
hard dependencies are accepted, output claims are available, and no current
responsibility record already owns the next transition.

### 3.5 Responsibility chain

Managed work uses an exact chain of durable and runtime identities:

1. **Assignment** selects one current owner for one ready Work Item.
2. **Dispatch** is the durable Room delivery outbox when a Room member must be
   started or notified.
3. **WorkBinding** is the runtime capability for the exact
   Execution/Plan/Work Item/Assignment/Attempt chain. A dispatched Room member
   receives it from the durable Dispatch/slot path. When the Room Lead assigns
   work to itself, the host signs the binding from the committed self Assignment
   mutation and installs it into that same physical round; Room membership or
   coordinator identity alone never substitutes for this receipt.
4. **Attempt** records one execution by an Agent or managed Subagent.
5. **Submission** immutably records the delivered result.
6. **Review Dispatch** selects and notifies a reviewer without creating a reviewer
   Assignment or reviewer Attempt.
7. **ReviewBinding** authorizes review of one exact immutable Submission.
8. **Acceptance** appends an `accepted`, `rejected`, or `changes_requested`
   decision.

There is never more than one current owner for a Work Item. A takeover terminates
the previous current chain before creating a fresh one.

An accepted Acceptance is the only fact that unlocks hard dependents. A succeeded
Attempt or a Submission alone is not acceptance. The accepted Review transaction
also creates or wakes one durable Execution completion-audit receipt. This receipt
does not assert readiness and grants no model authority; it only guarantees that
the backend will re-derive current blockers after the originating request or
process disappears.

### 3.6 History and evidence

Assignments, Attempts, Submissions, Acceptances, revisions, and domain events are
historical facts. Replacement actions append or supersede facts; they do not erase
them.

Runtime-only Agent activity, an unbound Room message, or a raw `@member` mention is
transport activity. It is not managed WorkGraph evidence unless the backend has
issued the exact WorkBinding or ReviewBinding required by the transition.

## 4. Plan authoring and materialization

### 4.1 Plan Document boundary

The authoring transport is one strict YAML string named `plan_document` using
`nexus_plan: 1`. The implemented operations are:

- `create`: create a new Execution and its first Plan;
- `replan`: add a new immutable Plan revision to the same Execution;
- `replace`: create a successor Execution and Plan under replacement fences.

The parser rejects unknown fields, missing required fields, invalid aliases,
duplicate logical identities, invalid dependency/output relationships, and
over-limit collections. Callers must use the parser-backed operation contract
rather than reconstructing a wider YAML format from prose.

The operation is selected only from the current `execution inspect` result. No
current Execution means `create`, including the first successor Plan after Goal
reset or retarget even when historical predecessor/successor metadata exists.
`replan` requires the returned current Execution and preserves that objective
boundary. `replace` requires a returned current transient Goal-free Execution.
Historical relationship language never substitutes for current state.

### 4.2 Prepare

`prepare_plan_execution`:

1. parses and normalizes the complete Plan Document;
2. validates static business and ownership constraints;
3. resolves the requested Goal boundary;
4. seals a durable proposal with an opaque `proposal_id` and digest;
5. atomically binds that exact proposal to the trusted
   owner/session/scope/coordinator key;
6. returns commit guidance without exposing machine proposal identifiers to the
   model.

Prepare does not create authoritative Work Items, activate a Plan, assign work, or
grant general coordination authority. It is allowed in Plan Mode because it only
produces a non-authoritative proposal. The durable binding is an explicit pointer,
not a query for the newest proposal. One physical round may own only one sealed
proposal; a second distinct prepare in that round is rejected. A newly inserted
prepare from a successor round supersedes prior sealed proposals in the same exact
binding scope; a late replay of a superseded prepare cannot move the pointer
backward, and a materializing proposal cannot be superseded.

### 4.3 Materialize

`plan_execution` normally accepts an empty input. The host resolves the exact
durable proposal binding and internally supplies its `proposal_id` and
`proposal_digest` to the materializer. It validates the sealed content, caller
authority, owner/session/scope, base Plan fences, Goal fences, and proposal
lifecycle before transactionally materializing it. During compatibility rollout,
an explicitly supplied legacy pair is accepted only when both fields are present
and exactly match the host binding; it never selects another proposal.

Materialization is idempotent for the same valid receipt. Replaying a materialized
binding returns the existing result; it does not duplicate the graph. The binding
survives physical-round and process boundaries, including Plan Mode exit.

`plan_execution` is rejected in Plan Mode because it mutates authoritative state
and may issue runtime coordination capability.

### 4.4 Goal boundary in Plan Documents

The parser recognizes the scalar Goal boundary values `none`, `current`, and
`inherit`:

- `create` accepts `none` or `current`. If omitted, it uses `current` only when the
  round carries exact Goal authority; otherwise it resolves to `none`.
- `replan` and `replace` inherit the existing boundary.
- `current` requires the exact current Goal identity and objective revision.

When creating a new Goal and a new bound WorkGraph in one user flow, Goal creation
must complete before Plan preparation. The two operations cannot safely run in
parallel because preparation needs the authoritative Goal identity and revision.

### 4.5 Live revision boundary

The implementation does **not** hot-carry live Assignment, Dispatch, WorkBinding,
Attempt, or ReviewBinding responsibility into a new active Plan revision. When the
new revision reuses the exact stable Work Item and spec, its latest reviewed
Submission and Acceptance remain authoritative satisfaction facts for that same
Execution; changing the spec fences those facts out.

Ordinary replan waits for a quiescent responsibility boundary. When explicitly
allowed to supersede active work, the transaction releases the old Assignment,
interrupts or cancels its live Attempt/dispatch chain, and activates the new
revision. That is teardown followed by fresh orchestration, not live responsibility
carry-over. Unreviewed Submission fences remain protected, and the WorkGraph canvas
keeps append-only lifecycle history across every revision of the same Execution.

## 5. Runtime authority and entry lanes

Tool availability is not authorization. Every mutation revalidates the exact
runtime capability and current SQL state.

| Entry lane | Authority | Allowed control-plane behavior |
| --- | --- | --- |
| Unbound ordinary Room round | Conversation identity plus, only for the durable Goal lead, a private start-of-round Goal revision snapshot | May chat and perform runtime-only activity; the Goal lead may mutate that exact Goal revision through the round-scoped command adapter, but cannot create managed evidence or mutate a Work Item without an Execution binding. |
| Exact coordinator round | Ephemeral `CoordinationBinding` for one Execution and round | May inspect and coordinate that Execution according to current state and tool-specific rules. |
| Exact worker round | `WorkBinding` for one responsibility chain | May act only on the bound Work Item and current Assignment/Attempt. |
| Exact reviewer round | `ReviewBinding` for one Submission | May review only that immutable Submission. |
| Managed Subagent | Child Attempt under the parent's WorkBinding | May execute only the same bound Work Item; its tools and result project beneath that Attempt. |
| Runtime-only Subagent | Runtime lineage without managed binding | May assist conversationally, but cannot satisfy WorkGraph delivery or review gates. |
| DM | May combine coordinator, self-worker, or reviewer identity | Still requires the same exact capability and state fences; DM is not an authorization bypass. |
| Plan Mode | Read plus proposal preparation | May read state and call `prepare_plan_execution`; authoritative Execution/Goal mutation and Agent execution remain blocked. |

External channel admission is a separate policy layer, not a substitute for these
fences. An admitted external DM may receive the Goal Skill and round-scoped Goal
command capability because Goal-only operation is a supported product mode;
capability issuance does not grant mutation authority. Invocations still need an
already-bound exact `GoalAuthorityState`, and external ingress cannot use the
trusted-visible-user late-bind exception for `retarget_goal`. Execution command
invocation remains denied by default on channel ingress and requires explicit
channel/Agent approval. If admitted, the round-scoped server still derives owner/session
identity and applies the same lane and SQL checks above.

Goal and Execution share one model-visible, always-loaded MCP tool:
`nexus.command`. The bundled Skills own model decisions; the host captures
owner, Agent, Session, Room role, Goal revision, WorkBinding, ReviewBinding and
coordination authority in a physical-round server instance. The model can submit
only `domain`, `action`, `operation`, closed `input`, `request_id` and the explicit
revision/digest fields used by Automation.

`contract` loads only the requested operation schema, `inspect` performs one
actor-filtered read, and `invoke` resolves the operation from the in-process
directory. Business input travels directly through the existing SDK `stream-json`
MCP call. Nexus creates no temporary input file, writable staging root, shell
process, environment capability, loopback broker or command shim. The bridge
replaces the per-round SDK server instance without restarting the runtime process.
The adapter validates required fields, closed objects, types, enums, patterns and
collection bounds before any domain handler or state read, then reuses the existing
typed result and receipt ledger.

The physical-round Execution context automatically projects only the current
actor's running or exceptional Runtime Graph nodes, Artifacts and exact control
returns. Bounded successful Tool/Subagent summaries remain durable but enter the
model only through explicit Execution `inspect`; ordinary rounds and mutation
results do not replay them.

### 5.1 Agent-facing structured command audit

DM and Room use the same tool. Host-bound identity is never accepted from model
input.

| Business operation | Exact tool sequence | Design boundary |
| --- | --- | --- |
| `get_goal` | `{"domain":"goal","action":"inspect"}` | Current Goal only. It has no mutation input. |
| `create_goal`, `retarget_goal`, `audit_objective_alignment`, `update_goal` | Goal inspect → exact contract → `invoke` with the business object in `input` and a stable `request_id` | Goal id, revision, owner, Agent, Room lead and round remain host facts. |
| `get_execution` (current or historical) | Execution inspect; historical reads add only `input.execution_id` | Historical selection is non-authorizing and never changes current Execution. |
| `prepare_plan_execution` | Execution inspect → exact contract → `invoke` with the complete Plan input | Strictly validates and durably seals a complete Plan proposal without materializing the WorkGraph. |
| `plan_execution` | Exact contract → `invoke` with `{}` and a stable `request_id` | Atomically materializes only the host-bound sealed proposal, preserving retry, CAS and Goal-revision fences. |
| Remaining Execution mutations | Execution inspect → exact contract → `invoke` with the current business input and stable `request_id` | Stable schemas remain visible; current service state and exact bindings decide authority. |

A stable `request_id` identifies one semantic intent and is reused for retries.
Changing operation, target or input requires a new ID. No transport path is exposed
to the model, so stale rounds cannot redirect input or carry authority forward.

### 5.2 Eight supported product entry paths

These are product entry paths, not eight tool variants. “Dialogue” means the user
asks the active model to create the structure; “Composer” means the trusted host
control above the input box accepts the Goal before a model continuation starts.

| Scope and user entry | Authoritative command sequence | Required resulting state |
| --- | --- | --- |
| DM · dialogue Goal | Goal inspect → exact `create_goal` contract → structured invoke | One standalone Goal. The DM Session Agent becomes the durable responsible Agent; no Execution is created. |
| DM · Composer Goal | Host `set_goal` transaction: create Goal → persist the exact `client_message_id` control record → start successor continuation; successor uses Goal inspect | One standalone Goal and one visible control record that never enters the model as an ordinary prompt. No model `create_goal` call occurs. |
| DM · dialogue WorkGraph | Execution inspect → exact `prepare_plan_execution` contract → invoke with `goal_binding=none` → exact `plan_execution` contract → invoke with `{}` | One transient Goal-free Execution with an authoritative Plan. |
| DM · dialogue Goal+WorkGraph | Complete `create_goal` first; after its applied receipt run the two Execution Plan commands with outer `goal_binding=current` | One Goal and one bilaterally confirmed Goal-bound Execution. Same-round dynamic authority carries the newly created Goal revision into Execution. |
| Room · dialogue Goal | Same structured Goal sequence as DM | One standalone Room Goal. The server-verified current Agent is persisted as creator and lead; other members can inspect but cannot mutate it. |
| Room · Composer Goal | Host Room `set_goal` transaction with exactly one verified selected lead → durable public control record → lead successor continuation uses Goal inspect | One standalone Room Goal with an exact lead and durable acceptance evidence. No model `create_goal` call occurs. |
| Room · dialogue WorkGraph | Same two-phase structured Execution sequence as DM with `goal_binding=none` | One transient Room Execution whose coordinator is host verified. Room members receive observation reads; only exact bindings authorize work or review mutations. |
| Room · dialogue Goal+WorkGraph | Lead completes `create_goal`, then the same round prepares and materializes with `goal_binding=current` | One confirmed Room Goal+Execution binding. The Goal lead/coordinator may coordinate; every member still needs its own WorkBinding or ReviewBinding to deliver or accept work. |

If a transient WorkGraph already exists before explicit Goal intent, neither DM nor
Room creates a parallel Goal and tries to bind it afterward. The coordinator uses
`promote_execution_to_goal` with the exact contract. If Composer created the Goal,
the accepted host request and model continuation are different physical rounds;
the successor receives the exact current revision at launch rather than inheriting
authority from the WebSocket request context.

After Goal reset or retarget, the predecessor Execution is historical. Until the
successor is materialized, Execution inspect has no current Execution, so both DM
and Room must seal the successor's first Plan as `operation: create` with
`goal_binding=current`. A one-shot `replan` rejection in this state is a caller
contract violation, not an expected retry phase.

The managed Goal/Execution Skill catalog is one shared Agent-service truth used
by Agent defaults, workspace deployment, the Skills catalog, prompts, and permission
policy. Existing Agents are migrated into that binding and cannot retain an old
Execution disable. The canonical Agent read model and the final runtime launch
projection both reassert the managed bindings, so a stale or concurrently restored
persistence row cannot place a managed Skill in `--disallowedTools`. A Room
aggregate may project member configuration for display,
but a physical Room round batch-loads its complete runtime profiles from the
canonical Agent service; the display projection never authorizes a runtime. The
round-scoped SDK server is replaced in process when those profiles or authorities
change, without expanding workspace write roots or restarting nxs.

Current `nexus.command` calls are control-plane transport, not independent
WorkGraph work. The runtime observer recognizes the exact managed tool identity and
persists only `domain + action + operation + request_id`, never business input. These
calls remain `detail` under their direct Agent owner even when they fail, retry, carry
an Artifact, or would otherwise look important. Read projection continues to classify
historical exact `${NEXUS_COMMAND_PATH}` calls, retired Goal/Execution MCP tools and
canonical owner-private command-input staging paths as the same transport detail;
compatibility reading never restores those routes or authority. A successful ordinary
Bash/PowerShell action stays
in details; only an active run, failure/cancellation/interruption, an exact Artifact, an
explicit visibility hint, a durable retry/loop-back edge, or a visible recovery after
failure promotes it onto the canvas.
External observable capability calls retain their semantic visibility. Review and
loop-back edges continue to use the structured Work Item/Agent and Gate facts rather
than a `submit_work` transport node. Each immutable Submission owns exactly one review
Gate; its Acceptance updates that Gate's decision and never creates a second current
snapshot Gate after the Assignment lease is cleared. Rejected or changes-requested
resubmissions remain distinct because each new Submission owns a new Gate. At each
assistant checkpoint, host-owned invoke
receipts still enrich candidate nodes in one graph read by exact
`domain + operation + request_id`; arbitrary shell output cannot recreate `assign_work`
segment authority or any other semantic operation identity. A provider Tool lifecycle
and host receipt carrying that same exact identity are one semantic assignment boundary,
even when checkpoint ordering persists two audit rows. Historical DM recovery may bind a
root Attempt without `agent_round_id` only when the exact request, coordinator identity,
and unique lifecycle interval all agree. `MEMORY.md` and `memory/` maintenance artifacts
remain available in run details but never promote a WorkGraph canvas node.

`get_execution` does not mutate durable Execution or Plan state. An unbound verified
member of the exact Room conversation receives a bounded shared WorkGraph
observation: objective, completion criteria, graph topology, and node status are
visible, while Assignment/Review/Submission evidence and every mutation action
remain absent. A member carrying an exact current WorkBinding or ReviewBinding keeps
that responsibility-scoped view; Room membership alone never downgrades or grants
such a binding. Observation never creates WorkBinding, ReviewBinding, Goal authority,
or coordination authority. On a verified current-coordinator read,
the same operation instead mints an ephemeral `CoordinationBinding` for the
current physical round in runtime memory. This is the explicit recovery path when
an existing WorkGraph continues in a new coordinator round; without it, the new
round has no coordination capability. The capability is not persisted and does
not make the read a graph mutation, but this coordinator-only runtime side effect
also means the operation must not be advertised with a pure `ReadOnly` annotation.

A successful `plan_execution` may also activate coordination for the current exact
round.

WorkBinding and ReviewBinding identify the complete Execution responsibility
chain. They do not carry a caller-supplied Goal ID. If the Execution is
Goal-bound, the backend derives and validates Goal identity from authoritative
storage.

Goal mutations require an exact Goal ID and objective revision at the mutation
boundary. An ambient current Goal, room membership, or visible Goal card is not
sufficient authority. Host-minted continuation and WorkGraph rounds carry a
shared runtime `GoalAuthorityState`. Separately, when a new physical round starts,
the host may resolve the current Goal for its durable responsible Agent: the
persisted Room lead, or the Agent encoded by a DM session key. That one exact
start-of-round revision is copied into a private round command authority state; it
is not written into the shared runtime context and therefore cannot become
ambient Execution/WorkGraph authority. Another Room member, a mismatched DM
Agent, a later revision, or a predecessor round still fails closed.

There is also one narrow objective-correction recovery lane: `retarget_goal` in a
trusted visible user DM or Room round may read the current Goal once at command
invocation, bind its exact ID and revision into the request, and then pass the
same service fences. This source exception does not authorize `update_goal`,
Objective Alignment, Execution mutation, internal continuations, external
ingress, or Agent-to-Agent handoffs.

## 6. Execution operations

The Execution operation directory exposes exactly 12 operations through round-scoped
`nexus.command`. Each operation owns one atomic control-plane
transition; `contract`, `inspect`, and `invoke` are transport actions, not extra
business operations.

| Operation | Atomic semantics |
| --- | --- |
| `get_execution` | Returns a bounded actor-specific view without changing durable Execution/Plan state. An exact Room member receives a full shared graph digest in an observation lane with only `get_execution` allowed; it carries no responsibility evidence or mutation authority. Bound workers/reviewers retain their responsibility-scoped view. A verified current-coordinator read mints coordination authority for the current physical round, so this is the explicit recovery entry and is not a pure `ReadOnly`-annotated operation. |
| `prepare_plan_execution` | Strictly parses, validates, normalizes, seals, and durably binds one non-authoritative proposal. It never creates authoritative graph state and is allowed in Plan Mode. |
| `plan_execution` | Resolves the host-owned exact proposal binding from empty model input, then materializes it as `create`, `replan`, or `replace` under CAS, identity, base-Plan, and Goal fences. It is transactional and idempotent; Plan Mode rejects it. |
| `abandon_execution` | Cancels a transient unbound Execution and atomically releases/cancels its live responsibility chain. A Goal-bound Execution must use the Goal retarget path instead. |
| `assign_work` | Assigns one ready Work Item to one owner and creates the pending root Attempt plus Room dispatch when required. It cannot create parallel current owners. |
| `submit_work` | Under an exact host-issued WorkBinding, omitted Work Item and Assignment locators default to that trusted binding; explicit values must match. In DM coordination or any other unbound round, `work_item_id` or `logical_key` is required while `assignment_id` remains optional. The mutation appends one immutable Submission and correlates/completes the current Attempt. Downstream hard dependencies remain locked until Acceptance. |
| `review_work` | Under an exact host-issued ReviewBinding, omitted Submission and Work Item locators default to that immutable review target; permitted self-review similarly defaults from an exact WorkBinding. Explicit values must match. In DM coordination or any other unbound round, at least one of `submission_id`, `work_item_id`, or `logical_key` is required and all supplied locators must agree. The mutation appends one Acceptance decision. Acceptance unlocks dependents; an accepted decision atomically wakes the durable backend completion audit, while rejection or changes requested preserves history and requires fresh work. When final acceptance completes an exactly Goal-bound Execution under current coordinator Goal authority, the mutation result routes the same physical round to Goal `audit_objective_alignment`. |
| `block_work` | Under an exact WorkBinding, omitted Work Item locators default to that binding and explicit values must match. The mutation moves the Work Item to `waiting_input` for a specific external input/authority blocker and rejects when an unreviewed Submission exists. |
| `resume_work` | Under an exact WorkBinding, omitted Work Item locators default to that binding and explicit values must match. The mutation records resolution/evidence and moves `waiting_input` back to `open`; it does not recreate an Assignment or revive an Attempt. |
| `take_over_work` | Coordinator-only atomic replacement: releases the old Assignment, interrupts/cancels its current chain, then creates a fresh Assignment/Attempt/dispatch. It rejects when an unreviewed Submission exists. |
| `audit_execution_alignment` | Appends an optional visible three-state objective-alignment gate to a current Execution. It does not transition work, reroute, retry, start a Goal, complete an Execution, or satisfy Goal `audit_objective_alignment`; terminal Execution rejects it. |
| `promote_execution_to_goal` | Binds a compatible transient Execution to a newly created durable Goal while preserving Plan and history. It enforces objective, state, configuration, authority, and exact binding fences; it does not copy the Plan. |

The model-visible operation contract is stable across bound and unbound rounds. Locator
fields therefore remain structurally optional: only the host knows whether the
current call carries an exact trusted binding, and the service enforces the
conditional one-of requirement at the mutation boundary. Tool availability,
`<assigned_work>`, and a graph node's `current_actor="true"` are state
projections, not WorkBinding or ReviewBinding receipts, and never authorize
locator omission by themselves.

A physical runtime round is an execution carrier, not an Assignment identity. One
round may serially complete root Attempts for different self-owned Assignments in
DM or Room coordination. In Room, every self-owned interval still begins with a
host-issued WorkBinding receipt and ends with an explicit responsibility transition;
the Room path never adopts DM's implicit assignment inference. The durable duplicate fence is therefore
`runtime_session_key + runtime_round_id + agent_round_id + assignment_id` for root
Attempts. A structured Room worker still receives one exact WorkBinding containing
its Execution, Plan, Work Item, Assignment, Attempt, and Dispatch identities; sharing
a physical round never grants access to a sibling Assignment. Child Attempts remain
distinguished by `parent_attempt_id + tool_use_id`.

If promotion commits the Execution binding but Goal-side confirmation is still
recovering, the tool returns `outcome: applied` (or idempotent `noop`) with
`goal_confirmation_status: pending` and an executable retry `next_action`. This
durable partial success is not a transport `IsError`.

Mutation results distinguish business rejection from obsolete responsibility.
When Goal retarget or Execution replacement has already closed the exact bound
predecessor, a late worker command returns `outcome: superseded` with
`reason_code: execution_terminal`. It is a successful transport carrying a muted
stop-old-round signal, not a failed submission and not evidence of Goal progress.
When a Plan preparation reaches the service with exact Goal authority that was
valid at physical-round launch but conflicts with the current Goal/Execution
revision, it returns `context_status: round_refresh_required` with no same-round
next action. `inspect` may expose the successor state but cannot mutate the old
round's authorization provenance; the old round terminates and the host-owned
successor continuation performs the next command.

Plain `context_status: refresh_required` is a same-round reread instruction and
never means that the physical round should wait for a successor. Mutation results
omit observed `runtime_facts` and may remove an optional `graph_digest`; explicit
Execution inspect keeps the bounded runtime history. Both paths must keep the
authoritative responsibility/review/action/blocker context even when that wire is
large; size alone must not fabricate either refresh status or end a round.

No operation may combine planning, assignment, execution, submission, and acceptance
into one implicit mutation. Command retries must use their stable request identity and receipt
idempotency identity where provided.

## 7. Goal operations

The Goal operation directory exposes exactly 5 operations through round-scoped
`nexus.command`. Goal-only operation remains valid; WorkGraph-specific gates apply
only to a `confirmed` managed binding.

| Operation | Atomic semantics |
| --- | --- |
| `get_goal` | Reads the current optional Goal, exact objective revision, backend-authoritative completion criteria, and usage state. The compact text and structured payload carry the same objective boundary so Objective Alignment never reconstructs current criteria from transcript, an old Plan, chat text, or workspace drafts. It does not mutate Goal or Execution state. |
| `create_goal` | Atomically creates a standalone active Goal only when no Goal exists for the scope and the objective is execution-ready. It never creates, reserves, or binds an Execution. A model-created Room Goal persists the server-verified creator as its lead; Room member count does not create a collaboration completion requirement. In DM, the session Agent is the responsible Agent. The creating round can mutate the new revision immediately, and later rounds of that same responsible Agent receive a private exact start-of-round Goal snapshot. If the same round already owns a compatible transient WorkGraph, explicit Goal intent instead uses `promote_execution_to_goal` with `persistence_requested`; when neither exists, a later `goal_binding=current` Plan materialization performs the bilateral binding. A token budget is set only when explicitly requested. It is rejected in Plan Mode. |
| `retarget_goal` | Applies an explicit user objective correction while preserving Goal identity and usage. A trusted visible user round may late-bind the exact current Goal/revision only for this tool; every other source requires existing Goal authority. `standalone`/`reserved` update the Goal revision directly; `confirmed` enters the successor rebase saga; `pending`/`conflict` fail closed. |
| `audit_objective_alignment` | Appends a three-state evidence report for the exact Goal revision and round without changing status. A Goal with a confirmed managed WorkGraph binding requires a current aligned report for completion; Goal-only and reserved Goals do not. |
| `update_goal` | Allows the model to mark the exact authorized Goal `complete` or `blocked`. The current Room lead or DM Agent decides when the objective is satisfied. Completion rechecks revision, binding resolution, Room in-flight work readiness, and, for a Goal with a confirmed managed WorkGraph binding, WorkGraph readiness plus current alignment evidence. Collaboration evidence is audit context and never a completion gate. Pause, resume, and limit controls remain user/system operations. |

### 7.1 Blocked policy boundary

The instruction to mark a Goal blocked only after the same concrete blocker
persists for at least three consecutive Goal turns is a **model behavior policy**.
The `update_goal` request must send a stable `blocker_id`, concrete `reason`, and
exact `needed_input`. The backend persists this typed recovery contract on the
blocked Goal and in its audit event, clears it when the Goal resumes or changes
objective, and exposes it to the UI. Reusing a `blocker_id` means the same
concrete blocker; it does not itself prove three consecutive turns. The backend
therefore does not claim to audit the persistence rule.

The backend does enforce exact Goal identity, objective revision, lead/authority,
and legal current state. Documentation, Skills, and prompts must not describe the
three-turn rule as a storage or service invariant.

### 7.2 Completion and retarget

Goal completion resolves binding first:

- `standalone` and `reserved` complete under Goal-only rules;
- `pending` and `conflict` fail closed;
- `confirmed` additionally requires the backend WorkGraph completion/readiness
  check (not `audit_execution_alignment`) and current Goal
  `audit_objective_alignment` evidence.
- every Room Goal completion additionally checks active Room slots, attributed
  handoffs, queues, and wakes so the lead cannot leave started Room work running
  behind a completed Goal. Member count and collaboration evidence do not affect
  completion eligibility. A public substantive non-lead reply may still be stored
  as monotonic audit provenance for the durable Goal ID; objective revision fences
  late writes without turning that provenance into authority or a gate.

The canonical mixed-mode closure is ordered across command domains: finish and
accept required Work Items; let final Acceptance and the completion reconciler
make the Execution terminal; consume the exact coordinator mutation receipt's
`goal/audit_objective_alignment` next action; then consume the aligned Goal
audit's `goal/update_goal` next action. The Goal audit remains independently
idempotent and may already exist for the same revision/round, preserving the old
MCP lifecycle semantics. It is never replaced by Execution
`audit_execution_alignment`, and a terminal Execution must not be mutated merely
to manufacture a Goal completion Gate. A rejected Goal completion returns a
domain-qualified recovery action: Goal audit for missing/stale alignment evidence,
or Execution inspection for unfinished managed work.

For a Goal with a confirmed managed WorkGraph binding, retarget is a successor saga rather than an in-place
graph edit. It reserves the successor relationship, materializes a fresh
Goal-fenced Execution/Plan, confirms the bilateral binding, and only then makes the
successor authoritative. Existing Work Items and responsibility history remain on
the predecessor and are not carried into the successor. If acceptance has already
made the predecessor Execution terminal while the durable Goal remains active, the
retarget saga preserves that terminal status and graph verbatim, appends one
idempotent old-revision-to-successor reservation event under an Execution-version
CAS, and admits the fresh successor through that exact event. A terminal predecessor is therefore not a
reason to trap an active Goal in a retry loop, and one old revision can never
reserve two different successors.

## 8. Transactions, retries, and reconciliation

### 8.1 SQL transaction boundary

Each authoritative transition writes its domain state and append-only Execution
events in one SQL transaction. A failure before commit exposes no partial
responsibility transition.

The required atomic groups are:

| Boundary | Must advance together | Why |
| --- | --- | --- |
| Goal mutation | Goal row version/status/objective revision, typed blocker or binding metadata, and Goal event | A reader must never observe a lifecycle state without its audit/recovery contract. |
| Continuation reservation | Goal continuation count/version, scheduling event, and complete server-only launch receipt | A crash must neither consume an unowned round nor launch an uncounted round. |
| Plan materialization/promotion | Execution/Plan aggregate mutation and pending Goal-confirmation receipt | Cross-domain Goal confirmation may lag, but it always has an exact durable recovery owner. |
| Responsibility command | Assignment/Attempt/Submission/Acceptance/Work Item state and Execution events/outboxes created by that command | Model-visible success must correspond to one complete responsibility fact. |
| Accepted review | Acceptance and pending completion-audit receipt | Process exit after review cannot strand a ready Execution. |
| Execution completion | Terminal Execution event/state and completion-audit settlement | Recovery cannot re-complete or leave a pending receipt behind a terminal graph. |

Round-local responsibility replacement happens synchronously from the successful
service receipt before the next command invocation returns to dispatch. It is not another
durable aggregate: every later call still revalidates the durable records. A
review/retarget/successor-plan chain therefore clears predecessor Review/Work
capabilities and binds the successor Execution in the same physical round, while
an old explicit identity returns a structured terminal/superseded outcome.

Materialization, assignment, submit, review, block, resume, takeover, abandon, and
terminal completion all re-read current state and enforce compare-and-set fences.
Callers must treat conflicts as a need to refresh, not as permission to overwrite.

An accepted Acceptance and its pending `execution_completion_audits` receipt are
committed in the same Review transaction. Foreground completion remains a separate
authoritative transition: it re-reads the latest active Plan and blocker set under
the Execution version fence. When completion commits, it marks the receipt
`completed` in that same transaction as the terminal Execution event. Startup,
post-commit mutation wakes, exact `next_attempt_at` timers, and a bounded audit
recover pending receipts, defer paused or blocked graphs, discard non-completed
terminal graphs, and retry `Complete` under fresh CAS. Migration
backfill creates receipts for legacy active managed graphs that already contain an
accepted current-Plan review. This recovery audit is backend readiness only; it
does not synthesize or replace `audit_execution_alignment` or Goal
`audit_objective_alignment` evidence.

Every Goal mutation that also emits Goal audit events updates the Goal row and
appends those events in one Goal repository transaction. Creation additionally
uses the partial unique current-Goal constraint; a concurrent winner is reported
as the stable Goal conflict instead of leaking a database unique error. A failure
before commit exposes neither the row version nor its events.

The user-visible host control record lives in the owner-scoped workspace ledger,
so it cannot share that SQL transaction. The Goal remains the authoritative state;
`chat_ack.user_message_committed` reports only whether the control record is
durable, and a failed ledger append must never be presented as a durable message.
The ordered failure contract is:

| Failure point | Goal SQL + events | visible `/goal` record | started/count | Goal title fallback | response | continuation |
| --- | --- | --- | --- | --- | --- | --- |
| validation or Goal SQL before commit | absent | absent | unchanged | unchanged | correlated command error | no |
| workspace ledger append | committed | absent | unchanged | already attempted from the committed Goal | transient ACK + finished host round | yes |
| conversation/session projection after ledger commit | committed | durable | may lag; this does not invalidate the record or Goal | already attempted from the committed Goal | durable ACK + finished host round | yes |
| socket response delivery | committed | according to ledger result | according to durable writes | already attempted from the committed Goal | delivery may be lost | yes, after the send attempt |
| normal path | committed | durable | started and count advanced | available immediately | durable ACK + finished host round | yes |

For a standalone or reserved Goal, explicit replacement is one Goal row/event
transition, including server-verified Room lead and collaboration metadata. A
confirmed WorkGraph retarget is necessarily the durable successor saga defined
above: the Goal identity is retained, while Execution/Plan move to a new successor;
it is not a cross-service SQL transaction.

### 8.2 Idempotency

Sealed proposal receipts, Execution/Goal confirmation receipts, stable dispatch
identities, Attempt/Submission correlation, and transition-specific idempotency
keys prevent retries from duplicating authoritative facts.

Idempotency means “same valid request returns the existing fact,” not “a stale
request becomes valid.” Identity, revision, authority, and terminal-state fences
still apply on replay.

### 8.3 SQL and Room delivery

SQL orchestration state and the Room workspace ledger are not one cross-store
transaction. Durable dispatch/review outboxes and reconciliation converge Room
delivery with SQL state.

The same rule applies to Goal-attributed collaboration: the source Goal remains
authoritative in Goal SQL, while directed messages, public handoffs, queue/wake,
target terminal, and source handback are separately durable, idempotent stages.
They carry exact owner/Goal/revision/root identities and reconcile forward; they
are never collapsed into one optimistic in-memory flag or treated as Goal
mutation authority.

The SQL transition remains authoritative. Late Room acknowledgements or runtime
results must be rejected when the Assignment, Attempt, Plan, Execution, Goal
revision, or binding is no longer current.

### 8.4 Runtime cancellation

The control plane always records local cancellation/interruption fences. A physical
provider interrupt is attempted only when the provider/session topology makes it
safe. When a shared provider session or multiple live rounds make physical
interrupt unsafe or unavailable, Nexus must not claim that the provider process
was interrupted; terminal fences still prevent late output from mutating state.

## 9. Current limits and explicit non-goals

1. **No live responsibility hot carry.** Plan revision never carries current
   Assignment, Dispatch, WorkBinding, Attempt, or ReviewBinding into the new
   revision. Exact stable Work Item/spec Submission and Acceptance facts remain
   visible across revisions of the same Execution; a changed spec cannot inherit them.
2. **No generic writable `control_edge`.** Plan dependencies are authored through
   the strict Plan Document. Runtime invoke/spawn/guard/loop-back/retry edges are
   observed read-only facts in the Execution Graph projection. Control outcomes
   use typed domain operations, not arbitrary edge writes.
3. **No backend three-turn blocker-count audit.** The consecutive-turn rule is
   model policy; backend validation covers stable blocker identity, recovery
   evidence, Goal identity, revision, authority, and state.
4. **No public planless WorkGraph.** An Execution without an active non-empty Plan
   is transitional internal state.
5. **No implicit managed evidence.** Runtime-only Subagents, ordinary Room
   messages, mentions, and unbound tool calls do not satisfy Work Item gates.
6. **No cross-store atomic Room delivery.** SQL plus durable outbox is
   authoritative; Room projection converges through reconciliation.
7. **No predecessor responsibility carry on Goal retarget.** A confirmed retarget
   creates a fresh successor graph and revalidates all work.

These are current product boundaries. Changing them requires coordinated protocol,
service, command contract, Skill/prompt, UI, migration, and test updates that preserve
the invariants below.

## 10. Required invariants

Implementations and callers must preserve all of the following:

1. Goal-only, WorkGraph-only, and Goal + WorkGraph remain independently valid.
2. Only an exact `confirmed` bilateral binding enables integrated Goal/WorkGraph
   behavior.
3. `pending` and `conflict` always fail closed for bound mutation.
4. One Execution has at most one active Plan revision.
5. One Work Item has at most one current Assignment owner.
6. Every managed worker mutation matches the exact WorkBinding chain.
7. Every review matches the exact immutable Submission and ReviewBinding.
8. A reviewer does not receive a worker Assignment or Attempt merely to review.
9. Only accepted Acceptance unlocks hard dependencies.
10. Historical Attempts, Submissions, Acceptances, and revisions are append-only.
11. Proposal preparation is non-authoritative; only exact receipt materialization
    creates or revises a managed graph.
12. Runtime capability never substitutes for current SQL state, and SQL state
    never substitutes for exact runtime authority.
13. Terminal and supersession fences classify late semantic runtime results as
    `superseded`, while other binding mismatches still reject fail closed. A
    physical Room Attempt terminal callback that arrives after its exact
    predecessor Execution was already atomically superseded is an idempotent
    no-op; it cannot resurrect work or produce a second binding failure.
14. Goal mutations use an exact Goal identity and objective revision. A durable
    responsible Agent may receive only the current start-of-round revision in a
    private round command authority state, while the trusted visible-user `retarget_goal`
    recovery lane acquires both at invocation; neither path weakens Execution,
    collaborator, stale-round, or stale-revision fences.
15. Managed Subagents remain children of the bound Work Item responsibility chain.
16. Plan revision and Goal retarget never silently carry responsibility history.
17. Field shapes and enums come from protocol/parser/schema truth sources rather
    than duplicated prose.

## 11. Change checklist

Any change to this control plane must update the relevant truth source and verify
the following layers together:

- protocol models and enums;
- strict Plan Document parser and parser-backed contract;
- storage transaction and reconciliation behavior;
- runtime coordination, WorkBinding, ReviewBinding, and Goal authority;
- `nexus.command` MCP input schemas, operation descriptions, and directory;
- bundled Skills and system prompts;
- actor-filtered HTTP/WS snapshots and the read-only Execution Graph projection;
- migrations and focused service/storage/MCP-command/frontend tests.

A change is incomplete when one layer advertises authority or state that another
layer cannot enforce.
