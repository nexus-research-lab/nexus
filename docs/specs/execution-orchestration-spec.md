# Execution Orchestration Control-Plane Specification

## 1. Purpose and scope

This spec covers the currently implemented control plane for Goal and managed
WorkGraph execution:

- the three product modes and the five Goal/Execution binding states;
- the user Goal control command and its continuation boundary;
- the durable Plan and responsibility model;
- the authority of coordinator, worker, reviewer, and Subagent rounds;
- the atomic semantics of the 12 Execution operations and 5 Goal operations;
- transaction, idempotency, reconciliation, and terminal-state invariants;
- current limits that callers and UI code must preserve.

The read-only runtime graph projection and its UI layout are owned by
[Execution Graph Specification](./execution-graph-spec.md).

### 1.1 Normative truth sources

This spec owns product semantics and cross-component invariants. Exact fields,
enums, parser rules, and `nexus.command` MCP contracts are defined in code
(paths under `internal/`):

| Concern | Truth source |
| --- | --- |
| Execution, Plan, Work Item, Assignment, Attempt, Submission, Acceptance, and events | [`protocol/execution.go`](../../internal/protocol/execution.go) |
| Plan proposal lifecycle and receipts | [`protocol/execution_plan_proposal.go`](../../internal/protocol/execution_plan_proposal.go) |
| Execution/Goal confirmation recovery receipts | [`storage/orchestration/goal_confirmation.go`](../../internal/storage/orchestration/goal_confirmation.go), [`service/orchestration/goal_confirmation_recovery.go`](../../internal/service/orchestration/goal_confirmation_recovery.go) |
| Goal and Goal/Execution binding states | [`protocol/goal.go`](../../internal/protocol/goal.go) |
| Goal/Execution cross-domain coordination | [`service/goalexecution/execution.go`](../../internal/service/goalexecution/execution.go), [`promotion.go`](../../internal/service/goalexecution/promotion.go); host adapters remain in `internal/app/goal` |
| Durable Goal continuation launch receipts | [`storage/goal/continuation_plan.go`](../../internal/storage/goal/continuation_plan.go), [`service/goal/continuation.go`](../../internal/service/goal/continuation.go) |
| Collection and projection limits | [`protocol/execution_limits.go`](../../internal/protocol/execution_limits.go) |
| Plan Document parsing and normalization | [`service/orchestration/plan_document.go`](../../internal/service/orchestration/plan_document.go) |
| Parser-backed Plan Document contract | [`service/orchestration/plan_document_contract.go`](../../internal/service/orchestration/plan_document_contract.go) |
| Proposal sealing and materialization | [`service/orchestration/plan_proposal.go`](../../internal/service/orchestration/plan_proposal.go), [`plan_materialization.go`](../../internal/service/orchestration/plan_materialization.go) |
| `nexus.command` MCP envelope, operation contract, and typed receipt | [`mcp/command/contract.go`](../../internal/mcp/command/contract.go), [`tool.go`](../../internal/mcp/command/tool.go), [`mcp/round_state.go`](../../internal/mcp/round_state.go) |
| Execution operation inputs, schemas, and directory | [`schema.go`](../../internal/mcp/command/execution/operation/schema.go), [`registry.go`](../../internal/mcp/command/execution/operation/registry.go) |
| Goal operation directory and contracts | [`mcp/command/goal/operation/registry.go`](../../internal/mcp/command/goal/operation/registry.go), [`mcp/command/goal/contract/contract.go`](../../internal/mcp/command/goal/contract/contract.go) |
| Round-scoped structured command adapter | [`mcp/command/tool.go`](../../internal/mcp/command/tool.go), [`app/runtime/command.go`](../../internal/app/runtime/command.go) |
| Verified runtime command facts | `orchestration.RuntimeCommandFact` is produced by `orchestration/runtimehook`; MCP receipt models do not enter the core package |
| Runtime responsibility and coordination authority | [`runtime/responsibility_authority.go`](../../internal/runtime/responsibility_authority.go), [`service/orchestration/work_binding.go`](../../internal/service/orchestration/work_binding.go), [`coordination_round.go`](../../internal/service/orchestration/coordination_round.go) |
| Accepted-review completion recovery receipts | [`storage/orchestration/completion_audit.go`](../../internal/storage/orchestration/completion_audit.go), [`service/orchestration/completion_audit_recovery.go`](../../internal/service/orchestration/completion_audit_recovery.go) |
| Background dispatch and recovery scheduling | [`infra/duework/loop.go`](../../internal/infra/duework/loop.go), [`service/orchestration/background_coordinator.go`](../../internal/service/orchestration/background_coordinator.go), [`storage/orchestration/background_deadline.go`](../../internal/storage/orchestration/background_deadline.go) |
| Room collaboration attribution and handoff recovery | [`protocol/room.go`](../../internal/protocol/room.go), [`storage/workspace/room_public_handoff.go`](../../internal/storage/workspace/room_public_handoff.go), [`service/room/realtime/public_handoff.go`](../../internal/service/room/realtime/public_handoff.go) |
| Host-owned command receipt classification | [`mcp/round_state.go`](../../internal/mcp/round_state.go), [`service/dm/goal_runtime.go`](../../internal/service/dm/goal_runtime.go), [`service/room/realtime/goal_runtime.go`](../../internal/service/room/realtime/goal_runtime.go) |
| UI/Slash Goal command dispatch and durable control history | [`service/slashcommand/goal.go`](../../internal/service/slashcommand/goal.go), [`service/dm/goal_command.go`](../../internal/service/dm/goal_command.go), [`service/room/realtime/goal_command.go`](../../internal/service/room/realtime/goal_command.go) |

Do not copy the Plan Document field list or operation input schemas into this
spec. When this spec and a source above disagree on a field shape or enum, the
source wins and this spec must be corrected.

### 1.2 生命周期实现归属

| 阶段 | 唯一业务归属 | 宿主适配边界 |
| --- | --- | --- |
| 创建目标 | Goal 校验目标、owner、版本与用量作用域；Execution 提供真实 binding 分类 | DM/Room 验证会话和 lead，各自持久化控制记录 |
| 规划与续跑 | Goal 持有 reservation、lease、claim、started、settle/retry 与重启恢复；Execution 持有 Plan 与责任状态 | DM/Room 在自身派发锁内重验显式输入优先级，再 claim 并启动；Room 另校验成员与协作阻塞 |
| 运行观察 | `orchestration/runtimehook.Observer` 统一事件观察与 compact 证据记录 | 宿主传入可信 actor；compact ID 用物理 Session 与 Agent round，不得换成共享 Room 根轮次 |
| 用量证据 | `goal/runtimeusage` 转换 provider result、assistant 与子任务消息；Goal accumulator 累计差额；`SubagentUsageObservation` 单调合并并确认落库 | DM/Room 保留 pending 容器、锁、原始观察时间与重试生命周期；旧回执不得清除新累计值或新终态证据 |
| 最终结算与交付 | Goal 持有 durable usage fence 与完成报告身份规则；状态完成与用量已结算分别判断 | DM 等待本轮 parent/child，Room 等待共享 scope 全部 slot；公私历史、输出授权与广播归宿主 |

- DM/Room 持有输入优先级、运行身份与输出权限。
- Goal 主包不依赖 runtime；运行协议转换位于 `runtimeusage` 子包，依赖方向为宿主 → 适配 → Goal。
- 共用值规则不移动宿主锁、不扩大事务、不增加独立恢复循环；禁止把宿主锁或 Room 公私输出策略下沉为通用流程。

## 2. Stable product model

### 2.1 Supported modes

Goal and WorkGraph are integrated only explicitly, never inferred from session
proximity.

| Mode | Durable state | Meaning |
| --- | --- | --- |
| Goal-only | Active Goal, no confirmed managed WorkGraph binding | Objective lifecycle, limits, continuation, blocking, and completion operate without a WorkGraph. |
| WorkGraph-only | Execution with an active materialized Plan and non-empty Work Items, no Goal fence | The managed graph executes independently. An ambient Goal in the same session is irrelevant. |
| Goal + WorkGraph | Active Goal and Execution with an exact confirmed bilateral binding | Goal objective/revision fences Plan authoring, retargeting, completion, and continuation. |

- Model `create_goal` and the host Goal command both create a `goal_only` Goal.
  Neither reserves nor bootstraps an Execution.
- A fresh Goal enters managed mode only when a sealed `goal_binding=current` Plan
  proposal begins materialization. That proposal owns the stable Execution
  identity, writes the Goal-side pending binding before the authoritative
  Execution/Plan mutation, and confirms the bilateral binding afterward.
- `goal_binding=none` never reads or mutates an ambient Goal.
- Historical `reserved` Goals and retarget successors remain recoverable as
  compatibility/transition states. They are not the default result of setting a Goal.
- An Execution row without an active materialized Plan is a bootstrap or
  reconciliation state, not a fourth mode. It must not be presented as a managed
  WorkGraph.

### 2.1.1 DM/Room entry convergence

Transport chooses owner, session, conversation, and responsible Agent; it never
chooses a different lifecycle. "Dialogue" means the user asks the active model;
"Composer" means the trusted host control above the input box accepts the Goal
before any model continuation starts. These are eight product entry paths, not
eight tool variants. Every model command loads its exact contract before invoke.

| Entry | Command sequence | Resulting state and authority |
| --- | --- | --- |
| DM · dialogue Goal | Goal inspect → `create_goal` (Goal service create transaction) | Goal-only, no Execution. The DM Session Agent owns the exact Goal revision. |
| DM · Composer Goal | Host `set_goal`: create Goal → persist the exact `client_message_id` control record → start successor continuation (uses Goal inspect) | Goal-only plus one visible control record, never a model prompt. No model `create_goal`. |
| DM · dialogue WorkGraph | Execution inspect → `prepare_plan_execution` (`goal_binding=none`) → `plan_execution` `{}` | WorkGraph-only transient Execution with an authoritative Plan; any ambient Goal is ignored. |
| DM · dialogue Goal+WorkGraph | `create_goal` (or retarget); after its applied receipt, both Plan commands with `goal_binding=current` | Goal + WorkGraph only after bilateral confirmation. Same-round dynamic authority carries the new revision into Execution; the round receives the confirmed responsibility receipt. |
| Room · dialogue Goal | As DM, with host-verified Room identity | Goal-only. The server-verified current Agent is persisted as creator and lead; only the lead mutates, others inspect. |
| Room · Composer Goal | Host Room `set_goal` with exactly one verified selected lead → durable public control record in the exact current conversation → lead successor continuation (uses Goal inspect) | Goal-only with an exact lead and durable acceptance evidence. No model `create_goal`. |
| Room · dialogue WorkGraph | As DM, `goal_binding=none` | Transient Room Execution with a host-verified coordinator. Members get observation reads; membership grants no Work Item authority. |
| Room · dialogue Goal+WorkGraph | Lead runs `create_goal` (or retarget), then in the same round prepares and materializes with `goal_binding=current` | Confirmed Room Goal+Execution binding. Lead/coordinator coordinates; each member still needs its own WorkBinding or ReviewBinding; coordinator, worker, and reviewer capabilities stay distinct. |

- `create_goal` never adopts an unbound transient Execution. If a transient
  WorkGraph exists before explicit Goal intent, neither DM nor Room creates a
  parallel Goal to bind later; the coordinator calls `promote_execution_to_goal`.
  New integrated work uses `goal_binding=current`. Goal creation therefore has no
  cross-domain half-commit.
- A Composer-accepted host request and its model continuation are different
  physical rounds. The successor receives the exact current revision at launch; it
  does not inherit authority from the WebSocket request context.
- After Goal reset or retarget, the predecessor Execution is historical. Until the
  successor materializes, Execution inspect has no current Execution, so DM and Room
  must seal the successor's first Plan as `operation: create` with
  `goal_binding=current`. A `replan` rejection in this state is a caller contract
  violation, not an expected retry phase.

### 2.1.2 State ownership and projections

Each concern has one authoritative state. A projection may lag or disappear; it
must never be used to manufacture the durable fact again.

| Concern | Authoritative truth | State transition |
| --- | --- | --- |
| Goal | `session_goals` plus append-only `goal_events` | Goal repository CAS on ID, owner, status, version, and objective revision |
| Plan/Execution/WorkGraph | Orchestration SQL aggregate: Execution, immutable Plan revision, Work Item state, Assignment, Attempt, Submission, Acceptance, and outbox/receipt rows | One service command re-reads the aggregate and commits one typed mutation |
| Responsibility capability | Durable responsibility records are business truth; one mutable in-memory `ResponsibilityAuthorityState` per physical round is its capability projection | A successful typed mutation receipt atomically replaces or clears Goal, Execution, Work, and Review authority for the round's next call |
| Collaboration handoff | Owner-scoped Room directed-message, public-handoff, and InputQueue ledgers with exact source Goal ID and objective revision | Schedule, dispatch, target terminal, and source handback are separate idempotent durable stages |
| Continuation | `goal_continuation_plans` open receipt plus Goal continuation count/event | `scheduled → claimed → started → settled`, or retry/release/cancel under revision and lease CAS |
| Progress | Typed `applied` mutation outcome under the exact current Goal/Execution responsibility; resulting Goal controller counters/events are durable | A counted mutation clears the no-progress streak; an exact handoff defers; a terminal empty/failure advances suppression |

Derived or recoverable projections:

- Goal: round command context, continuation strip, title/control history, UI blocker text.
- Plan/Execution: Execution Graph/UI, runtime nodes, Room delivery, snapshot caches.
- Responsibility: runtime command context and operation availability hints; neither
  authorizes without service revalidation.
- Collaboration: wake state, public feed, source continuation defer, collaboration audit evidence.
- Continuation: runtime running-round registration and UI continuation state.
- Progress: parsed SDK tool results are candidate facts only and never count by
  tool success alone.

Cache attribution is observational, not another control state. The usage ledger
stores provider-reported cache token totals beside low-cardinality lane/surface
fingerprints. These hashes compare stable versus changing host context; they are
neither provider cache keys nor authorization facts.

### 2.2 Goal/Execution binding states

The resolver exposes exactly five states. A Goal-free WorkGraph has no Goal binding
resolution at all; `standalone` is a Goal-side outcome, not the state of a
WorkGraph-only Execution.

| State | Meaning | Mutation policy |
| --- | --- | --- |
| `standalone` | The Goal has no Execution relationship. | Goal lifecycle is independent of WorkGraph state. |
| `reserved` | The Goal reserves an Execution identity, but the exact graph is not yet confirmed. | Goal lifecycle remains available; the reservation is provenance, not proof of a Plan. |
| `pending` | Binding materialization or confirmation is incomplete. | Bound operations fail closed until reconciliation confirms both sides. |
| `confirmed` | Goal and Execution agree on identity, owner, session, scope, and objective revision. | Integrated Goal + WorkGraph rules apply. |
| `conflict` | The two sides disagree or violate an identity/revision fence. | Bound operations fail closed and require reconciliation or an explicit correction path. |

- `reserved`, `pending`, and `confirmed` may be persisted in binding metadata;
  `standalone` and `conflict` are resolver outcomes.
- The resolver cross-checks Goal metadata against authoritative Execution storage.
  Callers must not classify a binding from one side alone.
- Only `confirmed` lets the managed graph participate in Goal completion, retarget,
  and continuation. A reserved Execution ID does not imply that an Execution,
  active Plan, or Work Item exists.

### 2.2.1 Goal continuation control state

Goal lifecycle status and automatic continuation are separate. An active Goal stays
`active` while its continuation is recovering or suppressed.

- Every Goal wire projection exposes server-derived
  `continuation_state=inactive|ready|recovering|suspended`. Clients must not
  reconstruct it from `empty_progress_count`, which is a durable consecutive
  no-progress streak, not a lifecycle status.
- The first empty continuation emits `continuation_recovery_scheduled` and gets one
  recovery turn with an explicit execute-now boundary, so a legitimate
  inspect-then-mutate sequence is not cut between stages.
- A second consecutive empty continuation emits `continuation_suppressed`. Automatic
  continuation stops until explicit Goal activity or Resume clears the streak.
- A provider/runtime terminal failure after a round starts stores `last_error` and
  suppresses immediately. A failure before runtime registration stays on the durable
  launch receipt and retries with bounded backoff, without changing Goal lifecycle.
- Any counted Goal mutation or explicit user activity clears the streak.

Progress counting:

- Read-only inspection is not progress, and tool transport success is not a
  progress receipt.
- Only an explicit `applied` Goal mutation, or an `applied` WorkGraph mutation under
  an exact current Goal binding, counts.
- Message sends, read/list/search operations, Task/Todo bookkeeping, unknown tools,
  rejected/no-op mutations, and unbound WorkGraph mutations fail closed (do not count).
- A durable exact Goal-attributed Room handoff or queue receipt defers the source
  continuation until handback. It is not mutation progress.

Continuation launch receipt:

- Scheduling is one durable transaction. The Goal continuation count and audit
  event advance together with a server-only `goal_continuation_plans` receipt that
  holds the exact Goal revision, Execution identity, previous round, purpose,
  prompt, and metadata.
- Prompt content is never copied into Goal metadata or client projections.
- A worker obtains a leased CAS claim, registers the exact runtime round, and only
  then advances the receipt to leased `started`. Duplicate workers cannot claim the
  same receipt.
- `started` is not a terminal launch receipt. It stays the recovery owner and the
  one-open-plan uniqueness fence until the exact runtime terminal callback advances
  it to `settled`.
- Room keeps two non-interchangeable identities through that callback: the
  outer/root continuation round settles the receipt; the slot `AgentRoundID`
  identifies progress, failure, and completion-miss audit events.
- A public handoff settles the source root receipt before waiting for the separate
  durable handback. The handback may then schedule a fresh continuation without
  waiting for lease recovery.
- A crash before claim leaves the scheduled receipt recoverable. A crash after claim
  or after runtime registration makes the same receipt recoverable after lease
  expiry, without incrementing the Goal again.
- Objective revision changes and non-active lifecycle transitions cancel old open receipts.
- Historical opaque reservation IDs are discarded: they lack enough authority and
  prompt data to replay safely. Migration refunds only distinct, non-empty
  outstanding reservation IDs from `continuation_count`, clamped at zero. Rounds
  that actually ran still count toward the same objective revision's usage limit.
- The recovery controller reconciles once at startup, wakes after local Goal or
  runtime receipt mutations, arms the exact next retry/lease deadline, and keeps a
  bounded low-frequency audit for lost hints and cross-process writes.

Goal confirmation receipt:

- Every SQL mutation that first makes an Execution Goal-bound writes one exact
  `execution_goal_confirmations` pending receipt in the same transaction. It stores
  Execution ID, Goal ID, objective revision, and completion criteria, so restart
  recovery depends on neither the originating request nor a Plan proposal row.
- Goal confirmation is idempotent. The background reconciler marks the receipt
  `confirmed` only after the Goal-side reverse binding succeeds.
- The Plan proposal confirmation state is its materialization-saga projection, not
  the sole recovery source.
- Goal confirmation, Plan proposal recovery, and completion audit share one
  deadline snapshot/driver but keep independent durable state machines and CAS
  transitions.

### 2.3 User Goal control command

Composer Goal mode and textual `/goal <objective>` are two transports for one Nexus
host command. It is intercepted before runtime input and must not be interpreted as
an ordinary model prompt. Ordered stages:

1. validate the authenticated DM or Room scope and, for Room, one current member
   as Goal lead;
2. best-effort normalize the objective inside a bounded portion of the request ACK
   window, then create or explicitly replace the current Goal under the
   current-Goal uniqueness, owner, objective-revision, and binding fences;
3. append one user-visible, terminal control record whose canonical content is
   `/goal <objective>` and whose subtype is `goal_set`;
4. attempt to send the durable/transient `chat_ack` and terminal
   `round_status=finished` for that host control round;
5. dispatch the active Goal continuation through the normal Goal state machine.

Ingress and delivery:

- WebSocket ingress completes cheap authenticated owner/session validation before
  acceptance. An accepted `set_goal` keeps its original connection FIFO position but
  runs in a bounded detached context that retains the authenticated context values.
- Closing the page, switching Session, or losing the socket may lose the response
  projection. It cannot cancel an accepted Goal mutation, control-record append,
  directory invalidation, or continuation attempt.
- The business deadline is authoritative. A separate short-lived delivery context
  may attempt the terminal ACK or correlated error after it, but cannot restart or
  extend the mutation.
- This detachment is not a durable command receipt; it does not make an in-flight
  command replayable after a server process crash.

Client reconciliation:

- The Web client holds the exact original Session binding and physical shared
  socket under the locally minted `client_request_id` until a raw ACK/error, an
  explicit destructive Session reset, or the bounded acceptance timeout.
- A timeout is `unknown`, not success or rejection.
- The control record must keep the exact `client_message_id` as durable acceptance
  evidence after ACK loss. Reconciliation accepts only that record, or a newer
  owner-scoped Goal identity/version whose `objective` or server-recorded
  `source_objective` matches the submission. The currently visible route, an
  unchanged same-objective Goal, and content or time proximity are not evidence.

Control record:

- It is not a runtime turn and never waits for an assistant/result terminal.
- It is a real visible user record: it starts a draft conversation, increments the
  session message count, and gives a new Goal-only session enough durable state for
  an immediate fallback title. Title generation and continuation therefore do not
  depend on a preceding ordinary prompt or the first model response.
- Title target: a standalone workspace session itself; for a Room-backed WebSocket
  DM, its canonical SQL conversation only. The current canonical objective is the
  synchronous fallback and also drives the normal concise-title generator.
- Existing user-defined titles are immutable.
- At startup, current Goals with durable owner provenance replay this projection
  once, repairing a missing or fallback title after a crash or lost response,
  without guessing a legacy owner.

Room-backed DM sessions (read model owned by [Room spec §5.4](./room-spec.md)):

- The SQL row also owns draft state and runtime settings; the Agent workspace
  session owns runtime message progress, last activity, context usage, and
  transcript lineage.
- Group conversation `message_count` is rebuilt from the canonical shared Room
  ledger and cached by ledger file version.
- Goal fallback and generated titles for a Room-backed DM update the SQL
  conversation only, never the workspace Session title.

Continuation timing:

- Continuation is scheduled only after the first control response send was
  attempted. Socket delivery success is not a prerequisite.
- When the model calls `create_goal` inside an already-running visible round, the
  hidden continuation waits until that round has fully left runtime and Goal usage
  accounting. A terminal UI event alone does not prove accounting cleanup.

Room collaboration attribution (persistence, recovery, and evidence rules owned by
[Room collaboration spec §4.5](./room-collaboration-spec.md)):

- A Room Goal continuation may request a conversation-only contribution through a
  public `@member` or a directed-message wake. The host attaches the exact Goal ID
  and objective revision across the directed-message fact, handoff ledger,
  InputQueue, and restart recovery. This is scheduling provenance, not
  `GoalAuthorityState`; the target round cannot call Goal mutation tools.
- The Room tool call carries a host-generated idempotency identity derived from the
  SDK tool-use identity, falling back to source round plus canonical input. Retries
  reuse the same directed-message and wake identity. Immediate and delayed wakes are
  scheduled durably before queue admission, retry online while pending, and stay
  complete after a late tool retry.
- While the handoff is pending, the source continuation is not classified as empty.
  At target terminal, substantive output resets the continuation run and the host
  schedules a fresh authorized lead continuation. Only public substantive output may
  become collaboration audit evidence, and that evidence never gates Goal completion.

## 3. Durable aggregate

### 3.1 Execution and Plan proposal

- An **Execution** is the durable container for one managed orchestration history.
  Its status is authoritative, and terminal transitions fence late work.
- A **Plan proposal** is a sealed, durable, non-authoritative authoring receipt with
  lifecycle `sealed`, `materializing`, `materialized`, `blocked`, or `discarded`. It
  cannot authorize work.
- Only `plan_execution` can materialize the exact sealed proposal into
  authoritative Execution/Plan state.

### 3.2 Plan revision

A **Plan** is an immutable revision; exactly one revision may be active per
Execution. Replan writes a new revision instead of mutating the old one. Superseded
and cancelled revisions remain historical facts.

Each revision-specific membership identifies a Work Item, its parent, whether it is
required and/or terminal, and its stable position. Exact fields live in protocol.

### 3.3 Work Item and delivery contract

A **Work Item** is the durable identity of a business responsibility. Its immutable
specification carries kind, subject, objective, acceptance contract, delivery
expectations, assignment policy, and other parser-validated business fields.

- Kinds are `produce`, `review`, `verify`, and `integrate`. They describe business
  responsibility, not runtime graph node types.
- Dependencies are revision-scoped. A hard dependency is satisfied only by an
  accepted upstream Submission; a soft dependency is visible context and does not
  gate readiness.
- Output claims are exclusive or shared. Exclusive claims prevent two active
  responsibility chains from owning the same declared output.
- Output scope forms: `file:<workspace-relative-path>`,
  `dir:<workspace-relative-path>`, or `semantic:<stable-key>`. Paths use forward
  slashes, are never absolute, never equal `.`, and never escape with `..`. An
  owner/Agent absolute workspace path is not a Plan scope.
- Claims are a durable scheduling and review contract, not a filesystem
  capability. Nexus validates Plan topology and exposes assigned scopes to the
  responsible runtime, but the workspace layer does not intercept every file write.
  `exclusive` means the orchestrator will not legitimately schedule conflicting
  lanes; it must not be described to users or models as an OS-enforced write lock.
  Plans that mutate shared workspace outputs should declare scopes.
- On an all-hard dependency path, overlapping exclusive claims are an ordered
  ownership handoff: each downstream Work Item stays locked until the upstream
  Submission is accepted, so a later draft, integration, or finalization step may
  declare the same output. Unrelated branches, siblings, parent nesting, and
  soft-only paths provide no such gate and still conflict unless every overlapping
  claim is shared.

### 3.4 Mutable Work Item state and derived lifecycle

Stored mutable Work Item state is limited to `open`, `waiting_input`, `cancelled`,
and `superseded`.

`ready`, `assigned`, `running`, `submitted`, and `accepted` are derived from the
active Plan, dependencies, and current responsibility records. They must not be
persisted as a second lifecycle that can drift from Assignment, Attempt,
Submission, or Acceptance history.

A Work Item is ready only when it belongs to the active Plan, is open, all hard
dependencies are accepted, output claims are available, and no current
responsibility record already owns the next transition.

### 3.5 Responsibility chain

Managed work uses an exact chain of durable and runtime identities:

1. **Assignment** selects one current owner for one ready Work Item.
2. **Dispatch** is the durable Room delivery outbox when a Room member must be
   started or notified.
3. **WorkBinding** is the runtime capability for the exact
   Execution/Plan/Work Item/Assignment/Attempt chain. A dispatched Room member
   receives it from the durable Dispatch/slot path. When the Room Lead assigns work
   to itself, the host signs the binding from the committed self Assignment and
   installs it into that same physical round, where Execution commands, Runtime
   Graph, and Subagent admission read it dynamically. Room membership or
   coordinator identity never substitutes for this receipt.
4. **Attempt** records one execution by an Agent or managed Subagent.
5. **Submission** immutably records the delivered result.
6. **Review Dispatch** selects and notifies a reviewer without creating a reviewer
   Assignment or Attempt.
7. **ReviewBinding** authorizes review of one exact immutable Submission.
8. **Acceptance** appends an `accepted`, `rejected`, or `changes_requested` decision.

- A Work Item never has more than one current owner. A takeover terminates the
  previous chain before creating a fresh one.
- An accepted Acceptance is the only fact that unlocks hard dependents. A succeeded
  Attempt or a Submission alone is not acceptance.
- The accepted Review transaction also creates or wakes one durable Execution
  completion-audit receipt (§8.1). It asserts no readiness and grants no model
  authority; it only guarantees the backend re-derives current blockers after the
  originating request or process disappears.

### 3.6 History and evidence

Assignments, Attempts, Submissions, Acceptances, revisions, and domain events are
historical facts. Replacement appends or supersedes facts; it never erases them.

Runtime-only Agent activity, an unbound Room message, or a raw `@member` mention is
transport activity. It is not managed WorkGraph evidence unless the backend issued
the exact WorkBinding or ReviewBinding the transition requires.

## 4. Plan authoring and materialization

### 4.1 Plan Document boundary

The authoring transport is one strict YAML string `plan_document` with
`nexus_plan: 1`. Operations:

- `create`: a new Execution and its first Plan;
- `replan`: a new immutable Plan revision on the same Execution;
- `replace`: a successor Execution and Plan under replacement fences.

The parser rejects unknown fields, missing required fields, invalid aliases,
duplicate logical identities, invalid dependency/output relationships, and
over-limit collections. Callers must use the parser-backed operation contract, not
a wider YAML format reconstructed from prose.

The operation is selected only from the current `execution inspect` result:

- No current Execution means `create`, including the first successor Plan after
  Goal reset or retarget, even when historical predecessor/successor metadata exists.
- `replan` requires the returned current Execution and preserves its objective boundary.
- `replace` requires a returned current transient Goal-free Execution.
- Historical relationship language never substitutes for current state.

### 4.2 Prepare

`prepare_plan_execution`:

1. parses and normalizes the complete Plan Document;
2. validates static business and ownership constraints;
3. resolves the requested Goal boundary;
4. seals a durable proposal with an opaque `proposal_id` and digest;
5. atomically binds that exact proposal to the trusted
   owner/session/scope/coordinator key;
6. returns commit guidance without exposing machine proposal identifiers to the model.

- Prepare creates no authoritative Work Item, activates no Plan, assigns no work,
  and grants no general coordination authority. It is therefore allowed in Plan Mode.
- The durable binding is an explicit pointer, not a query for the newest proposal.
- One physical round may own only one sealed proposal; a second distinct prepare in
  that round is rejected.
- A newly inserted prepare from a successor round supersedes prior sealed proposals
  in the same exact binding scope. A late replay of a superseded prepare cannot move
  the pointer backward, and a materializing proposal cannot be superseded.

### 4.3 Materialize

- `plan_execution` normally takes empty input. The host resolves the exact durable
  proposal binding and internally supplies its `proposal_id` and `proposal_digest`.
- It validates sealed content, caller authority, owner/session/scope, base Plan
  fences, Goal fences, and proposal lifecycle, then materializes transactionally.
- During compatibility rollout, an explicitly supplied legacy pair is accepted only
  when both fields are present and exactly match the host binding. It never selects
  another proposal.
- Materialization is idempotent for the same valid receipt: replaying a
  materialized binding returns the existing result without duplicating the graph.
  The binding survives physical-round and process boundaries, including Plan Mode exit.
- `plan_execution` is rejected in Plan Mode because it mutates authoritative state
  and may issue runtime coordination capability.

### 4.4 Goal boundary in Plan Documents

The parser recognizes the scalar Goal boundary values `none`, `current`, and `inherit`:

- `create` accepts `none` or `current`. If omitted, it uses `current` only when the
  round carries exact Goal authority; otherwise `none`.
- `replan` and `replace` inherit the existing boundary.
- `current` requires the exact current Goal identity and objective revision.

When one user flow creates a new Goal and a new bound WorkGraph, Goal creation must
complete before Plan preparation; preparation needs the authoritative Goal identity
and revision, so the two cannot run in parallel.

### 4.5 Live revision boundary

- A new active Plan revision never hot-carries live Assignment, Dispatch,
  WorkBinding, Attempt, or ReviewBinding responsibility.
- When the new revision reuses the exact stable Work Item and spec, its latest
  reviewed Submission and Acceptance remain authoritative satisfaction facts for
  that Execution. A changed spec fences those facts out.
- Ordinary replan waits for a quiescent responsibility boundary.
- When explicitly allowed to supersede active work, the transaction releases the old
  Assignment, interrupts or cancels its live Attempt/dispatch chain, and activates
  the new revision: teardown followed by fresh orchestration.
- Unreviewed Submission fences remain protected. The WorkGraph canvas keeps
  append-only lifecycle history across every revision of the same Execution.

## 5. Runtime authority and entry lanes

Tool availability is not authorization. Every mutation revalidates the exact
runtime capability and current SQL state.

| Entry lane | Authority | Allowed control-plane behavior |
| --- | --- | --- |
| Unbound ordinary Room round | Conversation identity; for the durable Goal lead only, a private start-of-round Goal revision snapshot | Chat and runtime-only activity. The Goal lead may mutate that exact Goal revision through the round-scoped adapter. No managed evidence and no Work Item mutation without an Execution binding. |
| Exact coordinator round | Ephemeral `CoordinationBinding` for one Execution and round | Inspect and coordinate that Execution per current state and tool rules. |
| Exact worker round | `WorkBinding` for one responsibility chain | Act only on the bound Work Item and current Assignment/Attempt. |
| Exact reviewer round | `ReviewBinding` for one Submission | Review only that immutable Submission. |
| Managed Subagent | Child Attempt under the parent's WorkBinding | Execute only the same bound Work Item; tools and result project beneath that Attempt. |
| Runtime-only Subagent | Runtime lineage without managed binding | Assist conversationally; cannot satisfy WorkGraph delivery or review gates. |
| DM | May combine coordinator, self-worker, or reviewer identity | Same exact capability and state fences; DM is not an authorization bypass. |
| Plan Mode | Read plus proposal preparation | Read state and call `prepare_plan_execution`; authoritative Execution/Goal mutation and Agent execution stay blocked. |

External channel admission is a separate policy layer, not a substitute for these fences:

- An admitted external DM may receive the Goal Skill and round-scoped Goal command
  capability, because Goal-only is a supported mode. Capability issuance grants no
  mutation authority: invocations still need an already-bound exact `GoalAuthorityState`.
- External ingress cannot use the trusted-visible-user late-bind exception for
  `retarget_goal`.
- Execution command invocation is denied by default on channel ingress and requires
  explicit channel/Agent approval. Even then, the round-scoped server derives
  owner/session identity and applies the same lane and SQL checks.

### 5.1 `nexus.command` transport

- Goal, Execution, Automation, and Subagent share one model-visible, always-loaded
  MCP tool, `nexus.command`. Bundled Skills own model decisions.
- The host captures owner, Agent, Session, Room role, Goal revision, WorkBinding,
  ReviewBinding, coordination authority, and Automation route in a physical-round
  server instance. The model submits only `domain`, `action`, `operation`, closed
  `input`, `request_id`, and the explicit revision/digest fields used by Automation.
- `contract` loads only the requested operation schema; `inspect` performs one
  actor-filtered read; `invoke` resolves the operation from the in-process directory.
  A mutation must load the fresh exact contract first.
- Business input travels directly through the SDK `stream-json` MCP call. Nexus
  creates no temporary input file, writable staging root, shell process,
  environment capability, capability token, loopback broker, or command shim.
- The bridge replaces the per-round SDK server instance in place, so authority
  updates need no runtime restart.
- The adapter validates required fields, closed objects, types, enums, patterns, and
  collection bounds before any domain handler or state read, then reuses the typed
  result and receipt ledger.
- Dynamic authority advanced by a same-round Goal mutation must be read atomically
  by the following Execution command.
- The physical-round Execution context projects only the current actor's running or
  exceptional Runtime Graph nodes, Artifacts, and exact control returns. Bounded
  successful Tool/Subagent summaries stay durable but reach the model only through
  explicit Execution `inspect`; ordinary rounds and mutation results never replay them.

### 5.2 Agent-facing command sequences

DM and Room use the same tool. Host-bound identity is never accepted from model
input. Read operations map to `inspect` and never go through `invoke`.

| Business operation | Exact tool sequence | Design boundary |
| --- | --- | --- |
| `get_goal` | `{"domain":"goal","action":"inspect"}` | Current Goal only; no input. |
| `create_goal`, `retarget_goal`, `audit_objective_alignment`, `update_goal` | Goal inspect → exact contract → `invoke` with business `input` and a stable `request_id` | Goal id, revision, owner, Agent, Room lead, and round remain host facts. |
| `get_execution` (current or historical) | Execution inspect; historical reads add only `input.execution_id` | Historical selection is non-authorizing and never changes the current Execution. |
| `prepare_plan_execution` | Execution inspect → exact contract → `invoke` with the complete Plan input | Strictly validates and durably seals a complete proposal without materializing the WorkGraph. |
| `plan_execution` | Exact contract → `invoke` with `{}` and a stable `request_id` | Atomically materializes only the host-bound sealed proposal under retry, CAS, and Goal-revision fences. |
| Remaining Execution mutations | Execution inspect → exact contract → `invoke` with current business input and stable `request_id` | Schemas stay visible; current service state and exact bindings decide authority. |

A stable `request_id` identifies one semantic intent and is reused for retries.
Changing operation, target, or input requires a new ID. No transport path is
exposed to the model, so stale rounds cannot redirect input or carry authority
forward.

### 5.3 Subagent transport and lifecycle

- The `subagent` domain reuses the fixed `nexus.command` schema. Operation contracts
  are disclosed on demand; the execution-orchestrator Skill carries the workflow.
- Nexus supplies a stable general-purpose child definition to nxs and keeps the
  native Agent tool hidden from the parent model catalog. This changes the initial
  deployment context once; launching children never mutates the tool schema or
  rewrites the prompt prefix.
- The bridge negotiates `subagent_control_v1`. During an active MCP call the host
  forwards the SDK-provided tool-use identity through a reentrant runtime control
  request. The runtime accepts it only in the current parent session, then runs the
  existing Agent/TaskOutput/TaskStop lifecycle and permission hooks.
- Plan Mode rejects spawn and send; reads and stopping remain available.
- Unsupported runtimes fail explicitly; there is no CLI fallback.
- A child cannot borrow the parent caller identity to recursively spawn another child.
- Spawn is asynchronous. Independent children may run concurrently and keep the
  original launch identity used by WorkGraph admission and terminal observation.
- An exact current Assignment creates a managed child Attempt; missing or ambiguous
  responsibility stays runtime-only. Child completion never substitutes for parent
  Submission, Review, or Acceptance.
- Subagent launch/lifecycle observations stay visible under the Runtime Graph rules;
  they are not classified as Goal/Execution control-only detail.
- Subagent mutation receipts deduplicate exact intent within one physical round,
  including errors and unknown outcomes. They are not durable restart receipts.
- After an unknown result, inspect existing tasks before any new mutation. Never
  infer non-execution from an absent response or replay with a new request identity.
  Read results expose only current-parent task projections.

### 5.4 Coordination recovery and Room observation

`get_execution` never mutates durable Execution or Plan state. Its result depends
on the caller:

- **Unbound verified member of the exact Room conversation**: a bounded shared
  WorkGraph observation in an observation lane where only `get_execution` is
  allowed. Objective, completion criteria, graph topology, and node status are
  visible; Assignment/Review/Submission evidence and every mutation action are absent.
- **Exact current WorkBinding or ReviewBinding holder**: keeps its
  responsibility-scoped view. Room membership alone never downgrades or grants a binding.
- **Verified current coordinator**: the read mints an ephemeral
  `CoordinationBinding` for the current physical round in runtime memory.

Observation never creates WorkBinding, ReviewBinding, Goal authority, or
coordination authority.

The coordinator read is the explicit recovery path when an existing WorkGraph
continues in a new coordinator round; without it, that round has no coordination
capability.

- The capability is not persisted and the read is not a graph mutation. Because of
  this coordinator-only side effect, the operation must not carry a pure `ReadOnly`
  annotation.
- The model invokes recovery through `nexus.command` with `domain=execution` and
  `action=inspect`. `get_execution` is a semantic name and is not invokable.
- The returned `execution_context` is XML text containing the lane and allowed actions.
- There is no user-facing coordination mode switch. Recovery alone requires no
  replanning or further user message.
- Terminal historical reads never mint or replace the current round's coordination
  capability.
- Transport rejections preserve a domain `reason_code` when available, without
  granting recovery authority.
- A successful `plan_execution` may also activate coordination for the current exact round.

### 5.5 Goal authority

- WorkBinding and ReviewBinding identify the complete Execution responsibility chain
  and carry no caller-supplied Goal ID. For a Goal-bound Execution, the backend
  derives and validates Goal identity from authoritative storage.
- Goal mutations require an exact Goal ID and objective revision at the mutation
  boundary. An ambient current Goal, room membership, or visible Goal card is not authority.
- Host-minted continuation and WorkGraph rounds carry a shared runtime `GoalAuthorityState`.
- When a new physical round starts, the host may resolve the current Goal for its
  durable responsible Agent: the persisted Room lead, or the Agent encoded by a DM
  session key. That one exact start-of-round revision is copied into a private round
  command authority state. It is not written into the shared runtime context, so it
  cannot become ambient Execution/WorkGraph authority.
- Another Room member, a mismatched DM Agent, a later revision, or a predecessor
  round fails closed.
- Narrow objective-correction lane: `retarget_goal` in a trusted visible user DM or
  Room round may read the current Goal once at invocation, bind its exact ID and
  revision into the request, and then pass the same service fences. This exception
  does not cover `update_goal`, Objective Alignment, Execution mutation, internal
  continuations, external ingress, or Agent-to-Agent handoffs.

### 5.6 DM continuation authority

DM and Room share the Goal/Execution/WorkGraph service but keep separate transport
identities. The shared control-plane binding is
`goal_id + objective_revision + execution_id`; the source boundary stays the exact
DM `session_key` or the exact Room `room_id + conversation_id`.

After the Goal service validates and claims a durable continuation plan, the DM
dispatcher may issue a host-only continuation authority for that physical round:

```text
owner_user_id
agent_id
scope_session_key       // exact structured DM session
goal_id
objective_revision
execution_id             // optional for Goal-only continuation
root_round_id
```

- The runtime command builder accepts it only when every field matches the live
  Agent, DM session, Goal authority, Responsibility authority, runtime lease, and
  current round. `execution_id` alone is never sufficient.
- The structured session must be a local WebSocket DM. Paired external IM, queue,
  echo, automation, and ordinary `agent_internal` rounds do not inherit it.
- An exact continuation authority selects the same full Execution operation registry
  as a trusted ordinary DM round.
- A missing, stale, cross-owner, cross-session, cross-revision, cross-Execution, or
  cross-round authority fails closed with an explicit identity or service error. It
  must not silently fall back to the WorkGraph authoring-only registry, which would
  make `summary=ready → verify=waiting` look valid without ever calling `assign_work`.
- Room rounds keep their verified Room source and membership boundaries; only exact
  WorkBinding/ReviewBinding or verified coordinator inspection authorizes
  mutations. Shared code may reuse the identity comparison helper, but no Room ID is
  accepted as a DM ID and no Room capability propagates to a DM round.
- The model-visible contract stays transport-neutral (§5.1). Locator fields such as
  `execution_id`, `work_item_id`, and `logical_key` identify business records; they
  cannot mint or expand host authority.
- `round_refresh_required` or an identity mismatch ends the old physical round,
  which then waits for a host-issued successor.

### 5.7 Skill catalog and control-plane observation

- The managed Goal/Execution Skill catalog is one shared Agent-service truth used by
  Agent defaults, workspace deployment, the Skills catalog, prompts, and permission
  policy. Existing Agents are migrated into that binding and cannot retain an old
  Execution disable.
- The canonical Agent read model and the final runtime launch projection both
  reassert the managed bindings, so a stale or concurrently restored persistence row
  cannot place a managed Skill in `--disallowedTools`.
- A Room aggregate may project member configuration for display, but a physical Room
  round batch-loads complete runtime profiles from the canonical Agent service; the
  display projection never authorizes a runtime. The round-scoped SDK server is
  replaced in process when profiles or authorities change, without expanding
  workspace write roots or restarting nxs.
- Goal/Execution `nexus.command` calls are control-plane transport, not WorkGraph
  work. The runtime observer recognizes the exact managed tool identity and persists
  only `domain + action + operation + request_id`, never business input. These calls
  stay `detail` under their direct Agent owner even when they fail, retry, or carry
  an Artifact.
- Legacy transport classification, receipt-based node enrichment, canvas
  promotion, review Gates, and DM self-assignment segments are owned by
  [Execution Graph Specification](./execution-graph-spec.md). Arbitrary shell output
  can never recreate `assign_work` segment authority or any other semantic
  operation identity.

## 6. Execution operations

The Execution directory exposes exactly 12 operations through round-scoped
`nexus.command`. Each owns one atomic control-plane transition; `contract`,
`inspect`, and `invoke` are transport actions, not business operations.

| Operation | Atomic semantics |
| --- | --- |
| `get_execution` | Bounded actor-specific read; no durable Execution/Plan change. A verified coordinator read mints round coordination authority (§5.4), so it is not a pure `ReadOnly` operation. |
| `prepare_plan_execution` | Strictly parse, validate, normalize, seal, and durably bind one non-authoritative proposal. Never creates authoritative graph state; allowed in Plan Mode. |
| `plan_execution` | Resolve the host-owned exact proposal from empty model input and materialize it as `create`, `replan`, or `replace` under CAS, identity, base-Plan, and Goal fences. Transactional and idempotent; Plan Mode rejects it. |
| `abandon_execution` | Cancel a transient unbound Execution and atomically release/cancel its live chain. A Goal-bound Execution must use Goal retarget instead. |
| `assign_work` | Assign one ready Work Item to one owner; create the pending root Attempt plus Room dispatch when required. Never creates parallel current owners. |
| `submit_work` | Append one immutable Submission and correlate/complete the current Attempt. Hard dependents stay locked until Acceptance. |
| `review_work` | Append one Acceptance decision (see below). |
| `block_work` | Move the Work Item to `waiting_input` for a specific external input/authority blocker. Rejected while an unreviewed Submission exists. |
| `resume_work` | Record resolution/evidence and move `waiting_input` back to `open`. Does not recreate an Assignment or revive an Attempt. |
| `take_over_work` | Coordinator-only atomic replacement: release the old Assignment, interrupt/cancel its chain, create a fresh Assignment/Attempt/dispatch. Rejected while an unreviewed Submission exists. |
| `audit_execution_alignment` | Append an optional visible three-state objective-alignment gate to a current Execution; terminal Execution rejects it. It transitions no work, reroutes, retries, starts no Goal, completes no Execution, and never satisfies Goal `audit_objective_alignment`. |
| `promote_execution_to_goal` | Bind a compatible transient Execution to a newly created durable Goal, preserving Plan and history without copying the Plan, under objective, state, configuration, authority, and exact binding fences. |

`review_work` outcomes:

- Acceptance unlocks dependents. An accepted decision atomically wakes the durable
  backend completion audit.
- Rejection or changes requested preserves history and requires fresh work.
- When final acceptance completes an exactly Goal-bound Execution under current
  coordinator Goal authority, the result routes the same physical round to Goal
  `audit_objective_alignment`.

### 6.1 Locators

- `submit_work`, `block_work`, `resume_work`: under an exact host-issued WorkBinding,
  omitted Work Item locators (and, for `submit_work`, the Assignment locator)
  default to that binding; explicit values must match.
- `submit_work` in DM coordination or any other unbound round requires
  `work_item_id` or `logical_key`; `assignment_id` stays optional.
- `review_work`: under an exact ReviewBinding, omitted Submission and Work Item
  locators default to that immutable review target; permitted self-review defaults
  from an exact WorkBinding. Explicit values must match. In an unbound round at least
  one of `submission_id`, `work_item_id`, or `logical_key` is required, and all
  supplied locators must agree.
- The model-visible contract is the same in bound and unbound rounds, so locator
  fields stay structurally optional. Only the host knows whether a call carries an
  exact trusted binding; the service enforces the conditional one-of at the
  mutation boundary.
- Tool availability, `<assigned_work>`, and a graph node's `current_actor="true"`
  are state projections, not binding receipts, and never authorize locator omission.

### 6.2 Root Attempt identity

- A physical runtime round is an execution carrier, not an Assignment identity. One
  round may serially complete root Attempts for different self-owned Assignments in
  DM or Room coordination.
- In Room, every self-owned interval begins with a host-issued WorkBinding receipt
  and ends with an explicit responsibility transition. Room never adopts DM's
  implicit assignment inference, and DM's same-round segmentation is never an
  implicit Room authorization.
- Root Attempt duplicate fence:
  `runtime_session_key + runtime_round_id + agent_round_id + assignment_id`.
- A structured Room worker receives one exact WorkBinding containing its Execution,
  Plan, Work Item, Assignment, Attempt, and Dispatch identities. Sharing a physical
  round never grants access to a sibling Assignment.
- Child Attempts are distinguished by `parent_attempt_id + tool_use_id`.

### 6.3 Results and refresh signals

- If promotion commits the Execution binding while Goal-side confirmation is still
  recovering, the tool returns `outcome: applied` (or idempotent `noop`) with
  `goal_confirmation_status: pending` and an executable retry `next_action`. This
  durable partial success is not a transport `IsError`.
- When Goal retarget or Execution replacement has already closed the exact bound
  predecessor, a late worker command returns `outcome: superseded` with
  `reason_code: execution_terminal`. This is a successful transport carrying a muted
  stop-old-round signal, not a failed submission and not Goal progress.
- When Plan preparation carries exact Goal authority that was valid at
  physical-round launch but conflicts with the current Goal/Execution revision, it
  returns `context_status: round_refresh_required` with no same-round next action.
  `inspect` may show successor state but cannot change the old round's authorization
  provenance; the old round terminates and the host-owned successor continuation
  issues the next command.
- Plain `context_status: refresh_required` is a same-round reread instruction; it
  never means the round should wait for a successor.
- Mutation results omit observed `runtime_facts` and may drop the optional
  `graph_digest`; inspect uses the same current-state projection (§5.1).
- Both keep the authoritative responsibility/review/action/blocker context
  even when the wire is large. Size alone must not fabricate a refresh status or end
  a round; only an explicit `round_refresh_required` from the service state machine
  ends the physical round.

No operation may combine planning, assignment, execution, submission, and acceptance
into one implicit mutation. Retries must reuse the stable request identity and the
receipt idempotency identity where provided.

## 7. Goal operations

The Goal directory exposes exactly 5 operations through round-scoped
`nexus.command`. Goal-only operation stays valid; WorkGraph-specific gates apply
only to a `confirmed` managed binding.

| Operation | Atomic semantics |
| --- | --- |
| `get_goal` | Read the current optional Goal, exact objective revision, backend-authoritative completion criteria, and usage state. No Goal or Execution mutation. |
| `create_goal` | Create a standalone active Goal only when no Goal exists for the scope and the objective is execution-ready. Never creates, reserves, or binds an Execution. Rejected in Plan Mode. |
| `retarget_goal` | Apply an explicit user objective correction, preserving Goal identity and usage. |
| `audit_objective_alignment` | Append a three-state evidence report for the exact Goal revision and round, without changing status. |
| `update_goal` | Mark the exact authorized Goal `complete` or `blocked`. |

- `get_goal`: the compact text and structured payload carry the same objective
  boundary, so Objective Alignment never reconstructs current criteria from
  transcript, an old Plan, chat text, or workspace drafts.
- `create_goal`:
  - A model-created Room Goal persists the server-verified creator as lead. Room
    member count creates no collaboration completion requirement. In DM, the session
    Agent is responsible.
  - The creating round can mutate the new revision immediately; later rounds of the
    same responsible Agent receive a private exact start-of-round snapshot (§5.5).
  - If the same round already owns a compatible transient WorkGraph, explicit Goal
    intent uses `promote_execution_to_goal` with `persistence_requested`. Otherwise a
    later `goal_binding=current` materialization performs the bilateral binding.
  - A token budget is set only when explicitly requested.
- `retarget_goal`: a trusted visible user round may late-bind the exact current
  Goal/revision for this tool only (§5.5); every other source needs existing Goal
  authority. `standalone`/`reserved` update the Goal revision directly; `confirmed`
  enters the successor saga (§7.2); `pending`/`conflict` fail closed.
- `audit_objective_alignment`: a Goal with a confirmed managed WorkGraph binding
  needs a current aligned report to complete; Goal-only and reserved Goals do not.
- `update_goal`: the current Room lead or DM Agent decides when the objective is
  satisfied. Completion checks follow §7.2. Pause, resume, and limit controls remain
  user/system operations.

### 7.1 Blocked policy boundary

- Marking a Goal blocked only after the same concrete blocker persists for at least
  three consecutive Goal turns is a **model behavior policy**, not a storage or
  service invariant. Docs, Skills, and prompts must not describe it as one.
- `update_goal` must send a stable `blocker_id`, a concrete `reason`, and exact
  `needed_input`. The backend persists this typed recovery contract on the blocked
  Goal and its audit event, clears it when the Goal resumes or changes objective,
  and exposes it to the UI.
- Reusing a `blocker_id` means the same concrete blocker; it does not prove three
  consecutive turns, and the backend does not audit that rule.
- The backend does enforce exact Goal identity, objective revision, lead/authority,
  and legal current state.

### 7.2 Completion and retarget

Goal completion resolves binding first:

- `standalone` and `reserved` complete under Goal-only rules.
- `pending` and `conflict` fail closed.
- `confirmed` additionally requires the backend WorkGraph completion/readiness check
  (not `audit_execution_alignment`) and current Goal `audit_objective_alignment` evidence.
- Every Room Goal completion also checks active Room slots, attributed handoffs,
  queues, and wakes, so the lead cannot leave started Room work running behind a
  completed Goal. Member count and collaboration evidence (audit provenance per the
  Room collaboration spec) do not affect eligibility.

Mixed-mode closure is ordered across command domains:

1. finish and accept the required Work Items;
2. final Acceptance and the completion reconciler make the Execution terminal;
3. consume the exact coordinator mutation receipt's `goal/audit_objective_alignment`
   next action;
4. consume the aligned Goal audit's `goal/update_goal` next action.

- The Goal audit stays independently idempotent and may already exist for the same
  revision/round. It is never replaced by Execution `audit_execution_alignment`.
- A terminal Execution must not be mutated merely to manufacture a Goal completion Gate.
- A rejected Goal completion returns a domain-qualified recovery action: Goal audit
  for missing/stale alignment evidence, Execution inspect for unfinished managed
  work. Models must not guess between similarly named operations.

For a Goal with a confirmed managed WorkGraph binding, retarget is a successor saga,
not an in-place graph edit:

- It reserves the successor relationship, materializes a fresh Goal-fenced
  Execution/Plan, confirms the bilateral binding, and only then makes the successor
  authoritative.
- Existing Work Items and responsibility history stay on the predecessor.
- If acceptance already made the predecessor Execution terminal while the Goal is
  still active, the saga preserves that terminal status and graph verbatim, appends
  one idempotent old-revision-to-successor reservation event under an
  Execution-version CAS, and admits the successor through that exact event. A
  terminal predecessor never traps an active Goal in a retry loop, and one old
  revision can never reserve two different successors.

## 8. Transactions, retries, and reconciliation

### 8.1 SQL transaction boundary

Each authoritative transition writes its domain state and append-only Execution
events in one SQL transaction. A failure before commit exposes no partial
responsibility transition. Required atomic groups:

| Boundary | Must advance together | Why |
| --- | --- | --- |
| Goal mutation | Goal row version/status/objective revision, typed blocker or binding metadata, and Goal event | A reader must never observe a lifecycle state without its audit/recovery contract. |
| Continuation reservation | Goal continuation count/version, scheduling event, and complete server-only launch receipt | A crash must neither consume an unowned round nor launch an uncounted round. |
| Plan materialization/promotion | Execution/Plan aggregate mutation and pending Goal-confirmation receipt | Cross-domain Goal confirmation may lag, but always has an exact durable recovery owner. |
| Responsibility command | Assignment/Attempt/Submission/Acceptance/Work Item state and the Execution events/outboxes it creates | Model-visible success must correspond to one complete responsibility fact. |
| Accepted review | Acceptance and pending completion-audit receipt | Process exit after review cannot strand a ready Execution. |
| Execution completion | Terminal Execution event/state and completion-audit settlement | Recovery cannot re-complete or leave a pending receipt behind a terminal graph. |

Round-local responsibility replacement:

- It happens synchronously from the successful service receipt, before the next
  command invocation returns to dispatch.
- It is not another durable aggregate; every later call still revalidates durable records.
- A review/retarget/successor-plan chain therefore clears predecessor Review/Work
  capabilities and binds the successor Execution in the same physical round, while
  an old explicit identity returns a structured terminal/superseded outcome.

Materialization, assignment, submit, review, block, resume, takeover, abandon, and
terminal completion all re-read current state and enforce compare-and-set fences.
Callers treat conflicts as a need to refresh, never as permission to overwrite.

Completion audit:

- An accepted Acceptance and its pending `execution_completion_audits` receipt are
  committed in the same Review transaction.
- Foreground completion is a separate authoritative transition. It re-reads the
  latest active Plan and blocker set under the Execution version fence and marks the
  receipt `completed` in the same transaction as the terminal Execution event.
- Startup, post-commit mutation wakes, exact `next_attempt_at` timers, and a bounded
  audit recover pending receipts: they defer paused or blocked graphs, discard
  non-completed terminal graphs, and retry `Complete` under fresh CAS.
- Migration backfill creates receipts for legacy active managed graphs that already
  contain an accepted current-Plan review.
- This recovery is backend readiness only. It never synthesizes or replaces
  `audit_execution_alignment` or Goal `audit_objective_alignment` evidence.

Goal repository:

- A Goal mutation that emits Goal audit events updates the row and appends those
  events in one Goal repository transaction; a failure before commit exposes
  neither the row version nor its events.
- Creation also relies on the partial unique current-Goal constraint. A concurrent
  winner is reported as the stable Goal conflict, never as a raw database unique error.
- For a standalone or reserved Goal, explicit replacement is one Goal row/event
  transition, including server-verified Room lead and collaboration metadata.
- A confirmed WorkGraph retarget is the durable successor saga (§7.2): Goal identity
  is retained while Execution/Plan move to a successor. It is not a cross-service
  SQL transaction.

Host control record:

The `/goal` control record lives in the owner-scoped workspace ledger, so it cannot
share the Goal SQL transaction. The Goal stays authoritative.
`chat_ack.user_message_committed` reports only whether the control record is
durable; a failed ledger append must never be presented as a durable message.

| Failure point | Goal SQL + events | visible `/goal` record | started/count | Goal title fallback | response | continuation |
| --- | --- | --- | --- | --- | --- | --- |
| validation or Goal SQL before commit | absent | absent | unchanged | unchanged | correlated command error | no |
| workspace ledger append | committed | absent | unchanged | already attempted from the committed Goal | transient ACK + finished host round | yes |
| conversation/session projection after ledger commit | committed | durable | may lag; does not invalidate the record or Goal | already attempted from the committed Goal | durable ACK + finished host round | yes |
| socket response delivery | committed | according to ledger result | according to durable writes | already attempted from the committed Goal | delivery may be lost | yes, after the send attempt |
| normal path | committed | durable | started and count advanced | available immediately | durable ACK + finished host round | yes |

### 8.2 Idempotency

Sealed proposal receipts, Execution/Goal confirmation receipts, stable dispatch
identities, Attempt/Submission correlation, and transition-specific idempotency
keys prevent retries from duplicating authoritative facts.

Idempotency means "the same valid request returns the existing fact", not "a stale
request becomes valid". Identity, revision, authority, and terminal-state fences
still apply on replay.

### 8.3 SQL and Room delivery

- SQL orchestration state and the Room workspace ledger are not one cross-store
  transaction. Durable dispatch/review outboxes and reconciliation converge Room
  delivery with SQL state.
- Goal-attributed collaboration follows the same rule: the source Goal stays
  authoritative in Goal SQL, and the handoff stages (§2.1.2) carry exact
  owner/Goal/revision/root identities and reconcile forward. They are never collapsed
  into one optimistic in-memory flag or treated as Goal mutation authority.
- The SQL transition stays authoritative. Late Room acknowledgements or runtime
  results are rejected when the Assignment, Attempt, Plan, Execution, Goal revision,
  or binding is no longer current.

### 8.4 Runtime cancellation

The control plane always records local cancellation/interruption fences. A physical
provider interrupt is attempted only when the provider/session topology makes it
safe. When a shared provider session or multiple live rounds make it unsafe or
unavailable, Nexus must not claim the provider process was interrupted; terminal
fences still stop late output from mutating state.

## 9. Current limits and explicit non-goals

1. **No live responsibility hot carry** across Plan revisions (§4.5).
2. **No generic writable `control_edge`.** Plan dependencies come only from the
   strict Plan Document. Runtime invoke/spawn/guard/loop-back/retry edges are
   read-only facts in the Execution Graph projection; control outcomes use typed
   domain operations, not arbitrary edge writes.
3. **No backend three-turn blocker-count audit** (§7.1).
4. **No public planless WorkGraph.** An Execution without an active non-empty Plan
   is transitional internal state.
5. **No implicit managed evidence.** Runtime-only Subagents, ordinary Room messages,
   mentions, and unbound tool calls do not satisfy Work Item gates.
6. **No cross-store atomic Room delivery.** SQL plus durable outbox is
   authoritative; Room projection converges through reconciliation.
7. **No predecessor responsibility carry on Goal retarget.** A confirmed retarget
   creates a fresh successor graph and revalidates all work.

Changing any of these requires coordinated protocol, service, command contract,
Skill/prompt, UI, migration, and test updates that preserve the invariants below.

## 10. Required invariants

1. Goal-only, WorkGraph-only, and Goal + WorkGraph remain independently valid.
2. Only an exact `confirmed` bilateral binding enables integrated Goal/WorkGraph behavior.
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
12. Runtime capability never substitutes for current SQL state, and SQL state never
    substitutes for exact runtime authority.
13. Terminal and supersession fences classify late semantic runtime results as
    `superseded`; other binding mismatches still reject fail closed. A physical Room
    Attempt terminal callback arriving after its exact predecessor Execution was
    atomically superseded is an idempotent no-op; it cannot resurrect work or
    produce a second binding failure.
14. Goal mutations use an exact Goal identity and objective revision. The private
    start-of-round snapshot and the visible-user `retarget_goal` late bind (§5.5)
    never weaken Execution, collaborator, stale-round, or stale-revision fences.
15. Managed Subagents remain children of the bound Work Item responsibility chain.
16. Plan revision and Goal retarget never silently carry responsibility history.
17. Field shapes and enums come from protocol/parser/schema truth sources, not
    duplicated prose.

## 11. Change checklist

A control-plane change must update the relevant truth source and verify these
layers together:

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

## 实现约束

- exact Room conversation 的协作者只能读 Goal 标识、共享 WorkGraph 的目标/完成标准/拓扑/节点状态（§5.4）；观察视图不含 Assignment、Review、Submission 证据，只读观察不得授予 Assignment、Review、Submission、Plan mutation、Goal 或 coordination capability。旧 round、旧 revision、后台/外部来源仍必须 fail closed。
- 完成态 WorkGraph 的抽取、durable Draft 版本、隐藏编辑 Session、保存确认与内置模板合同见 [Slash 命令规范](./slash-command-spec.md) 与 [执行图规范](./execution-graph-spec.md)。本规范只保留以下边界：
  - UI 确认保存直接把已生成草图与命令名/标题/描述交给宿主事务，不再启动后台模型 round；图结构只来自 exact durable Draft。
  - 命名图永不保存 Tool、运行身份、Assignment、Attempt、结果、Artifact、Submission、Review 或 Acceptance。
  - 系统内置模板由 `internal/service/workgraphworkflow` 版本化维护，只读，不写 owner 数据、不伪造来源 Execution。
  - 固定 `/workgraph` 只启用当前请求协作；内置模板与 owner 命名图每次复用都创建 fresh Execution/Plan/Work Item identity，不新增通用 WorkGraph MCP。
