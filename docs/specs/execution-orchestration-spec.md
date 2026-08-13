# Execution Orchestration Control-Plane Specification

## 1. Purpose and scope

This document describes the control plane that Nexus implements today for Goal and
managed WorkGraph execution. It defines:

- the three supported product modes and the five Goal/Execution binding states;
- the user-visible Goal control command and its continuation boundary;
- the durable Plan and responsibility model;
- the authority carried by coordinator, worker, reviewer, and Subagent rounds;
- the atomic semantics of the 12 Execution tools and 5 Goal tools;
- transaction, idempotency, reconciliation, and terminal-state invariants;
- current limits that callers and UI code must preserve.

It does **not** define the read-only runtime graph projection or its UI layout.
Those are specified in [Execution Graph Specification](./execution-graph-spec.md).
Only currently implemented control-plane behavior is in scope.

### 1.1 Normative truth sources

This specification owns product semantics and cross-component invariants. Exact
fields, enums, parser rules, and MCP JSON schemas remain code-generated or
code-defined truth:

| Concern | Truth source |
| --- | --- |
| Execution, Plan, Work Item, Assignment, Attempt, Submission, Acceptance, and events | [`internal/protocol/execution.go`](../../internal/protocol/execution.go) |
| Plan proposal lifecycle and receipts | [`internal/protocol/execution_plan_proposal.go`](../../internal/protocol/execution_plan_proposal.go) |
| Execution/Goal confirmation recovery receipts | [`internal/storage/orchestration/goal_confirmation.go`](../../internal/storage/orchestration/goal_confirmation.go), [`internal/service/orchestration/goal_confirmation_recovery.go`](../../internal/service/orchestration/goal_confirmation_recovery.go) |
| Goal and Goal/Execution binding states | [`internal/protocol/goal.go`](../../internal/protocol/goal.go) |
| Collection and projection limits | [`internal/protocol/execution_limits.go`](../../internal/protocol/execution_limits.go) |
| Plan Document parsing and normalization | [`internal/service/orchestration/plan_document.go`](../../internal/service/orchestration/plan_document.go) |
| Parser-backed Plan Document contract | [`internal/service/orchestration/plan_document_contract.go`](../../internal/service/orchestration/plan_document_contract.go) |
| Proposal sealing and materialization | [`internal/service/orchestration/plan_proposal.go`](../../internal/service/orchestration/plan_proposal.go), [`plan_materialization.go`](../../internal/service/orchestration/plan_materialization.go) |
| Execution MCP inputs, schemas, and registry | [`internal/mcp/execution/tool/input.go`](../../internal/mcp/execution/tool/input.go), [`schema.go`](../../internal/mcp/execution/tool/schema.go), [`registry.go`](../../internal/mcp/execution/tool/registry.go) |
| Goal MCP registry and tool contracts | [`internal/mcp/goal/tool/registry.go`](../../internal/mcp/goal/tool/registry.go) |
| Runtime responsibility and coordination authority | [`internal/service/orchestration/work_binding.go`](../../internal/service/orchestration/work_binding.go), [`coordination_round.go`](../../internal/service/orchestration/coordination_round.go) |
| UI/Slash Goal command dispatch and durable control history | [`internal/service/slashcommand/goal.go`](../../internal/service/slashcommand/goal.go), [`internal/service/dm/goal_command.go`](../../internal/service/dm/goal_command.go), [`internal/service/room/realtime/goal_command.go`](../../internal/service/room/realtime/goal_command.go) |

Do not copy the complete Plan Document field list or MCP input schema into this
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

An Execution row without an active materialized Plan is a bootstrap or
reconciliation state. It is not a fourth public product mode and must not be
presented as a managed WorkGraph.

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

Every SQL mutation that first makes an Execution Goal-bound writes one exact
`execution_goal_confirmations` pending receipt in the same transaction. The
receipt stores Execution ID, Goal ID, objective revision, and completion criteria;
therefore restart recovery does not depend on the originating request or on a
Plan proposal row. Goal confirmation is idempotent, and the background reconciler
marks the receipt `confirmed` only after the Goal-side reverse binding succeeds.
The Plan proposal confirmation state remains its materialization-saga projection,
not the sole recovery source.

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
attempted; socket delivery success is not a continuation prerequisite. When MCP
`create_goal` creates a Goal inside an already-running visible model round, its
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
the continuation run, public substantive output may satisfy the Room-visible
collaboration evidence gate, and the host schedules a fresh authorized lead
continuation. No-reply, error, interruption, stale-revision output, and private
output never manufacture public collaboration evidence.

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
   Execution/Plan/Work Item/Assignment/Attempt chain.
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
Attempt or a Submission alone is not acceptance.

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
over-limit collections. Callers must use the parser-backed contract and MCP schema
rather than reconstructing a wider YAML format from prose.

### 4.2 Prepare

`prepare_plan_execution`:

1. parses and normalizes the complete Plan Document;
2. validates static business and ownership constraints;
3. resolves the requested Goal boundary;
4. seals a durable proposal with an opaque `proposal_id` and digest;
5. returns the exact receipt required for materialization.

Prepare does not create authoritative Work Items, activate a Plan, assign work, or
grant general coordination authority. It is allowed in Plan Mode because it only
produces a non-authoritative proposal.

### 4.3 Materialize

`plan_execution` accepts the exact `proposal_id` and `proposal_digest`. It validates
the sealed content, caller authority, owner/session/scope, base Plan fences, Goal
fences, and proposal lifecycle before transactionally materializing it.

Materialization is idempotent for the same valid receipt. Replaying a materialized
receipt returns the existing result; it does not duplicate the graph.

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

The implementation does **not** hot-carry Assignment, Dispatch, WorkBinding,
Attempt, Submission, ReviewBinding, or Acceptance into a new active Plan revision.

Ordinary replan waits for a quiescent responsibility boundary. When explicitly
allowed to supersede active work, the transaction releases the old Assignment,
interrupts or cancels its live Attempt/dispatch chain, and activates the new
revision. That is teardown followed by fresh orchestration, not carry-over.
Unreviewed Submission fences remain protected.

## 5. Runtime authority and entry lanes

Tool availability is not authorization. Every mutation revalidates the exact
runtime capability and current SQL state.

| Entry lane | Authority | Allowed control-plane behavior |
| --- | --- | --- |
| Unbound ordinary Room round | Conversation identity plus, only for the durable Goal lead, a private start-of-round Goal revision snapshot | May chat and perform runtime-only activity; the Goal lead may mutate that exact Goal revision through `nexus_goal`, but cannot create managed evidence or mutate a Work Item without an Execution binding. |
| Exact coordinator round | Ephemeral `CoordinationBinding` for one Execution and round | May inspect and coordinate that Execution according to current state and tool-specific rules. |
| Exact worker round | `WorkBinding` for one responsibility chain | May act only on the bound Work Item and current Assignment/Attempt. |
| Exact reviewer round | `ReviewBinding` for one Submission | May review only that immutable Submission. |
| Managed Subagent | Child Attempt under the parent's WorkBinding | May execute only the same bound Work Item; its tools and result project beneath that Attempt. |
| Runtime-only Subagent | Runtime lineage without managed binding | May assist conversationally, but cannot satisfy WorkGraph delivery or review gates. |
| DM | May combine coordinator, self-worker, or reviewer identity | Still requires the same exact capability and state fences; DM is not an authorization bypass. |
| Plan Mode | Read plus proposal preparation | May read state and call `prepare_plan_execution`; authoritative Execution/Goal mutation and Agent execution remain blocked. |

External channel admission is a separate policy layer, not a substitute for these
fences. An admitted external DM may expose the five Goal tools by default,
including alignment audit, because Goal-only operation is a supported product
mode; exposure does not grant mutation authority. Such calls still need an
already-bound exact `GoalAuthorityState`, and external ingress cannot use the
trusted-visible-user late-bind exception for `retarget_goal`. Execution tools
remain denied by default on channel ingress and require explicit channel/Agent
approval. If admitted, they still receive only server-derived owner/session
identity and must pass the same lane and SQL checks above.

`get_execution` does not mutate durable Execution or Plan state. On a verified
current-coordinator read it mints an ephemeral `CoordinationBinding` for the
current physical round in runtime memory. This is the explicit recovery path when
an existing WorkGraph continues in a new coordinator round; without it, the new
round has no coordination capability. The capability is not persisted and does
not make the read a graph mutation, but this runtime side effect also means the
tool must not be advertised with a pure `ReadOnly` annotation.

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
start-of-round revision is copied into a private `nexus_goal` authority state; it
is not written into the shared runtime context and therefore cannot become
ambient Execution/WorkGraph authority. Another Room member, a mismatched DM
Agent, a later revision, or a predecessor round still fails closed.

There is also one narrow objective-correction recovery lane: `retarget_goal` in a
trusted visible user DM or Room round may read the current Goal once at tool
invocation, bind its exact ID and revision into the request, and then pass the
same service fences. This source exception does not authorize `update_goal`,
Objective Alignment, Execution mutation, internal continuations, external
ingress, or Agent-to-Agent handoffs.

## 6. Execution tools

The Execution MCP registry exposes exactly 12 tools. Each tool owns one atomic
control-plane transition.

| Tool | Atomic semantics |
| --- | --- |
| `get_execution` | Returns an actor-filtered current snapshot without changing durable Execution/Plan state. A verified current-coordinator read mints coordination authority for the current physical round, so this is the explicit recovery entry and is not a pure `ReadOnly`-annotated tool. |
| `prepare_plan_execution` | Strictly parses, validates, normalizes, and seals a non-authoritative proposal. It never creates authoritative graph state and is allowed in Plan Mode. |
| `plan_execution` | Materializes the exact sealed receipt as `create`, `replan`, or `replace` under CAS, identity, base-Plan, and Goal fences. It is transactional and idempotent; Plan Mode rejects it. |
| `abandon_execution` | Cancels a transient unbound Execution and atomically releases/cancels its live responsibility chain. A Goal-bound Execution must use the Goal retarget path instead. |
| `assign_work` | Assigns one ready Work Item to one owner and creates the pending root Attempt plus Room dispatch when required. It cannot create parallel current owners. |
| `submit_work` | Under the exact WorkBinding, appends one immutable Submission and correlates/completes the current Attempt. Downstream hard dependencies remain locked until Acceptance. |
| `review_work` | Under the exact ReviewBinding or permitted self-review path, appends one Acceptance decision for the selected Submission. Acceptance unlocks dependents; rejection or changes requested preserves history and requires fresh work. |
| `block_work` | Moves the bound Work Item to `waiting_input` for a specific external input/authority blocker and terminates the current Attempt chain. It rejects when an unreviewed Submission exists. |
| `resume_work` | Records resolution/evidence and moves `waiting_input` back to `open`. It does not recreate an Assignment or revive an Attempt. |
| `take_over_work` | Coordinator-only atomic replacement: releases the old Assignment, interrupts/cancels its current chain, then creates a fresh Assignment/Attempt/dispatch. It rejects when an unreviewed Submission exists. |
| `audit_execution_alignment` | Appends an optional visible three-state objective-alignment gate. It does not transition work, reroute, retry, start a Goal, or complete an Execution. |
| `promote_execution_to_goal` | Binds a compatible transient Execution to a newly created durable Goal while preserving Plan and history. It enforces objective, state, configuration, authority, and exact binding fences; it does not copy the Plan. |

A physical runtime round is an execution carrier, not an Assignment identity. One
round may serially complete root Attempts for different self-owned Assignments in
DM or Room coordination. The durable duplicate fence is therefore
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

No tool may combine planning, assignment, execution, submission, and acceptance
into one implicit mutation. Tool retries must use their stable receipt or
idempotency identity where provided.

## 7. Goal tools

The Goal MCP registry exposes exactly 5 tools. Goal-only operation remains valid;
WorkGraph-specific gates apply only to a `confirmed` managed binding.

| Tool | Atomic semantics |
| --- | --- |
| `get_goal` | Reads the current optional Goal and usage state. It does not mutate Goal or Execution state. |
| `create_goal` | Creates an active Goal only when no Goal exists for the scope and the objective is execution-ready. A model-created Room Goal persists the server-verified creator as its lead and derives the multi-member collaboration requirement from the owner-scoped Room directory; in DM, the session Agent is the responsible Agent. The creating round can mutate the new revision immediately, and later rounds of that same responsible Agent receive a private exact start-of-round Goal snapshot. When the same round already owns a compatible transient WorkGraph, the explicit Goal flow reuses and binds that Execution instead of creating a second graph. A token budget is set only when explicitly requested. It is rejected in Plan Mode. |
| `retarget_goal` | Applies an explicit user objective correction while preserving Goal identity and usage. A trusted visible user round may late-bind the exact current Goal/revision only for this tool; every other source requires existing Goal authority. `standalone`/`reserved` update the Goal revision directly; `confirmed` enters the successor rebase saga; `pending`/`conflict` fail closed. |
| `audit_objective_alignment` | Appends a three-state evidence report for the exact Goal revision and round without changing status. A Goal with a confirmed managed WorkGraph binding requires a current aligned report for completion; Goal-only and reserved Goals do not. |
| `update_goal` | Allows the model to mark the exact authorized Goal `complete` or `blocked`. Completion rechecks revision, binding resolution, current Room membership/work readiness, and, for a Goal with a confirmed managed WorkGraph binding, WorkGraph readiness plus current alignment evidence. A current multi-member Room always requires room-visible non-lead collaboration evidence even when a legacy or alternate creation path lacks the cached requirement metadata. Pause, resume, and limit controls remain user/system operations. |

### 7.1 Blocked policy boundary

The instruction to mark a Goal blocked only after the same concrete blocker
persists for at least three consecutive Goal turns is a **model behavior policy**.
The `update_goal` input does not send blocker identity or a three-turn history, and
the backend does not audit that persistence rule.

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
- every Room Goal completion also reloads the owner-scoped Room member directory;
  more than one distinct Agent member requires public, substantive non-lead
  collaboration evidence from anywhere in the same durable Goal lifecycle.
  Once observed for the same Goal ID, this evidence is monotonic across objective
  retargets, consecutive lead rounds, lead reassignment, and temporary changes to
  whether collaboration is required. Objective revision still fences late event
  attribution, but does not invalidate an already committed collaboration fact.
  The persisted requirement is a cached fence, not the sole source of
  the current member fact.
  App-server may create a Room Goal only in a non-complete state, so creation and
  completion cannot be collapsed into one request that precedes this gate.

For a Goal with a confirmed managed WorkGraph binding, retarget is a successor saga rather than an in-place
graph edit. It reserves the successor relationship, materializes a fresh
Goal-fenced Execution/Plan, confirms the bilateral binding, and only then makes the
successor authoritative. Existing Work Items and responsibility history remain on
the predecessor and are not carried into the successor.

## 8. Transactions, retries, and reconciliation

### 8.1 SQL transaction boundary

Each authoritative transition writes its domain state and append-only Execution
events in one SQL transaction. A failure before commit exposes no partial
responsibility transition.

Materialization, assignment, submit, review, block, resume, takeover, abandon, and
terminal completion all re-read current state and enforce compare-and-set fences.
Callers must treat conflicts as a need to refresh, not as permission to overwrite.

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
   Assignment, Dispatch, WorkBinding, Attempt, Submission, ReviewBinding, or
   Acceptance into the new revision.
2. **No generic writable `control_edge`.** Plan dependencies are authored through
   the strict Plan Document. Runtime invoke/spawn/guard/loop-back/retry edges are
   observed read-only facts in the Execution Graph projection. Control outcomes
   use typed domain tools, not arbitrary edge writes.
3. **No backend three-turn blocker audit.** The consecutive-turn rule is model
   policy; backend validation covers identity, revision, authority, and state.
4. **No public planless WorkGraph.** An Execution without an active non-empty Plan
   is transitional internal state.
5. **No implicit managed evidence.** Runtime-only Subagents, ordinary Room
   messages, mentions, and unbound tool calls do not satisfy Work Item gates.
6. **No cross-store atomic Room delivery.** SQL plus durable outbox is
   authoritative; Room projection converges through reconciliation.
7. **No predecessor responsibility carry on Goal retarget.** A confirmed retarget
   creates a fresh successor graph and revalidates all work.

These are current product boundaries. Changing them requires coordinated protocol,
service, MCP schema, Skill/prompt, UI, migration, and test updates that preserve
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
    private `nexus_goal` state, while the trusted visible-user `retarget_goal`
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
- MCP input schemas, tool descriptions, and registry;
- bundled Skills and system prompts;
- actor-filtered HTTP/WS snapshots and the read-only Execution Graph projection;
- migrations and focused service/storage/MCP/frontend tests.

A change is incomplete when one layer advertises authority or state that another
layer cannot enforce.
