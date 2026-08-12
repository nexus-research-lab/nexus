# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added a complete conversational configuration control plane across owner, Agent-self, Room-host, and Room-member contexts, with database-backed scope enforcement, redacted inspect/plan/apply/history tools, native human approval and secret entry, resource-version CAS, durable audit/reconcile state, authorization flows, immediate revocation, and explicit hot-reload timing.
- Added four independently selected built-in configuration Skills for progressive role guidance, so only the current trusted role is discoverable in each DM or Room runtime while the backend remains the authority for every operation.
- Added owner-scoped private Skill sources with server-side JSON search, encrypted Bearer credentials, checksum-verified ZIP imports, online update checks, and owner-only conversational create/update/delete/import through native secret slots, shared catalog-version CAS, and write-after-read verification.
- Added adaptive buffered Markdown streaming with stable frame-paced rendering while preserving conversation scroll ownership.
- Added conversational Room member pause/resume control with Room-version CAS, authority-epoch fencing, active-task interruption, pending-work recovery, and write-after-read verification for owner-main and current-host contexts.
- Added an opt-in emotion system preference, disabled by default, that controls whether per-round emotion context is sent to Agents.
- Added task-owned scheduled-automation permissions with global Agent-default seeding, owner-scoped persistent approval cards, one-run and task-level grants, connector reauthorization, side-effect-aware explicit retry, script hash binding, and Main Session run/revision continuity without weakening existing task behavior.

### Fixed

- Repaired Goal token accounting when providers returned `total_tokens: 0` alongside positive usage fields: new DM/Room parent totals now fall back to the complete breakdown, existing Goal and all-Agent parent ledgers are migrated, historical completion receipts refresh from the corrected aggregate, and genuine all-zero usage remains authoritative.
- Allowed the final Room or DM session tab to close into the new-conversation page while preserving its conversation history.
- Reconnected Room Goal continuation across public mentions, directed-message wakes, busy queues, and restart recovery with an exact host-only Goal revision attribution: collaborator rounds remain unable to mutate Goal state, while terminal replies hand control back to a fresh authorized lead continuation instead of leaving an active Goal stuck in no-progress suppression. Goal-directed wakes now establish a deterministic handoff before immediate/delayed dispatch, repair interrupted directed-message-to-handoff writes only for the current revision, release failed startup claims, preserve the logical root through queue recovery, deduplicate MCP retries with a host command identity, and durably retry both immediate and delayed wakes without reviving completed work. Target terminal and Goal handback are separate durable stages, startup can repair pre-attribution terminal roots only from exact Goal audit/root/history facts, user cancellation never auto-resumes, and the Goal strip distinguishes an automatic stop from a real user pause with the reason inline.
- Restored a durable Goal completion receipt on the final assistant reply, with hidden exact Goal/round binding and only authoritative elapsed time and finalized actual-token totals; pending or unavailable usage now stays visually absent instead of appearing as zero or an error state.
- Made the WorkGraph partial warning apply only after runtime visibility projection, so hidden detail activity no longer consumes canvas capacity or reports a false truncation while display-worthy overflow still remains explicit.
- Removed the redundant standalone/reserved Goal binding badge while retaining the server-owned binding state for clear authorization and WorkGraph transitions.
- Unified Composer Goal mode and textual `/goal` behind one host command that creates or retargets the Goal, persists a terminal `/goal …` control record, starts and titles new Goal-only sessions (including their canonical DM conversations), bounds objective normalization inside the ACK window, and only then dispatches Goal continuation instead of sending the objective as an ordinary model prompt.
- Kept Goal-only titles, started state, and message progress stable after refresh by writing Room-backed titles only to the authoritative SQL conversation and merging it with the Agent workspace runtime projection instead of replacing progress with the unused legacy `messages` count; group conversation counts now rebuild from canonical Room history and cache by ledger version.
- Closed Goal state and history races by suppressing hidden continuation while the visible round or its usage guard is still live, allowing only trusted visible user rounds to late-bind the exact current revision for `retarget_goal`, rejecting `update_goal` as a creation fallback, and preserving control-before-continuation order without synthesizing an interrupted assistant for completed host controls.
- Made Goal creation and every event-bearing Goal mutation commit the row version and audit events in one optimistic SQL transaction, normalized concurrent current-Goal insertion to a stable conflict, and folded server-verified Room lead/collaboration changes into the same replacement mutation instead of leaving partially updated Goal state.
- Simplified the WorkGraph canvas header to its title during normal execution, while retaining one lifecycle badge for waiting, paused, or terminal graphs and projection-trust warnings.
- Made Execution-to-Goal confirmation recoverable across crashes and restarts with a transactionally persisted exact binding receipt, background reconciliation, and idempotent finalization; adaptive promotion now reports durable `applied`/`noop` plus an explicit pending confirmation and retry action instead of a misleading transport failure.
- Made `prepare_plan_execution` Goal binding explicit with `none`, `current`, and `inherit`, so Goal-free WorkGraphs never absorb an ambient session Goal and Goal-bound proposals require exact round Goal authority.
- Separated Goal-only, Goal-free WorkGraph, and confirmed Goal-bound WorkGraph lifecycles across REST, app-server, MCP, DM, and Room entry points: reserved identities no longer trigger managed gates, pending/conflicting bindings fail closed, confirmed bindings retain WorkGraph completion and retarget coordination, user retargets automatically dispatch an exact successor-planning continuation, and Goal/Execution tools share one exact per-round authority that upgrades only from a confirmed service receipt without ambient Goal discovery.
- Scoped Goal current, usage, event, and app-server reads to durable server-owned Goal provenance; WebSocket Goal subscriptions are now registered only after successful authorization and isolated by both owner and thread, preventing rejected calls or colliding thread IDs from receiving another owner's updates.
- Kept the expanded task panel centered over its summary and prevented the Composer Goal label from wrapping vertically in narrow layouts.
- Repaired databases that had already applied the scheduled-task permission pipeline under its former migration 71 by recognizing only the complete legacy schema, preserving tasks, runs, and approval requests, recording it as migration 86, and replaying the official private-Skill migration 71 in order; legacy tasks still receive their permission-policy compatibility backfill on startup.
- Restored the request-scoped configuration-secret draft boundary so conversational approvals keep secret values in memory, clear them on request changes, and submit only complete server-declared slots.
- Routed Automation, Workspace, and Goal operations to their specialized conversation tools while keeping Agent, Room, Session, Provider, Channel, Connector, Skill, preference, and emotion changes behind one inspect → plan → native approval → CAS apply → verify workflow instead of direct database or config-file edits.
- Closed the main-Agent-in-Group-Room configuration bypass and made Room tool inheritance monotonic: a Room policy can no longer remove an Agent deny or expand an explicit Agent allowlist.
- Recovered a stable Execution identity when an external Goal continues, allowing DM and Room Goals to start instead of failing before runtime launch.
- Made `NEXUS_RUNTIME_ISOLATION_MODE` the sole runtime isolation selector, so authenticated local Web development can deliberately use `off` or `audit` while `enforce` retains its platform and launcher checks.
- Hid composed runtime-owned Execution and Goal context carriers from DM history while preserving hidden continuation round alignment, eliminating leaked internal prompts and phantom empty replies.
- Completed Session, Room, Agent, scheduled-task, and imported-Skill cleanup across runtime, SQL, and owner-scoped files; transcript lineage stays in existing Session metadata so deletion leaves no orphaned data.
- Closed an Agent runtime before deleting its session and removed the complete owner-scoped transcript artifact graph, including summaries and unshared Subagent transcripts, so deleted sessions no longer leave runtime data behind.
- Drained the exact blocked DM or Room attempt before resuming an approved scheduled task, denied every later tool from that blocked attempt, bound approval and explicit retry to the rendered job/run/request/policy snapshot, and carried the same evidence boundary through Main Session events; task cards now keep permission state as the primary attention reason, expose actions only with a matching actionable request, move technical detail behind one entry, hide the prior permission error while the resumed attempt is active, and animate the running identity without overriding reduced-motion preferences.
- Sequenced explicit Goal creation before its first WorkGraph proposal across the stable prompt, Goal/Execution Skills, and both MCP tool contracts, so Agents no longer launch `create_goal` and `prepare_plan_execution` in parallel while the backend continues to reject stale ambient-Goal races.
- Repaired legacy databases that had already recorded Execution migration 61 before `goal_execution_identity_claims` was added, allowing startup proposal reconciliation and Goal-to-WorkGraph identity recovery to run instead of failing against a missing table.
- Made Plan boundary authority explicit: a fresh `operation: create` under an active Goal now seals the exact server-owned Goal objective even when the provider omits or paraphrases the transport field, while Goal-free create/replace still require a document objective, replan still inherits the current Execution boundary, and true Goal-to-existing-Execution objective conflicts remain rejected.
- Let `create_goal` and WorkGraph planning converge in the same round by durably reserving one deterministic Execution identity before any Plan exists; sealed proposals, Goal continuations, and materialization now reuse that exact fence, while previously created explicit Goals with a missing `execution_id` recover the same reservation from their server-owned command instead of deadlocking on `goal_binding_conflict`.
- Unified Nexus Plan Document v1 parser fields, MCP schema guidance, repair results, and the bundled orchestration Skill around one exact contract; invalid YAML now returns every allowed and required field, common alias corrections, and a parser-valid example so Agents rewrite the complete document once instead of discovering `subject`, `objective`, `deliverable`, `depends_on`, and `acceptance_criteria` through repeated rejected calls.
- Restored every durable child Attempt as an independent Subagent WorkGraph node even when the operational Snapshot retains only its latest terminal child; native Agent task history now recovers exact task identity, avatar metadata, completion status, and child Tool ownership, while the launch Tool remains correlation evidence instead of overwriting the child lifecycle.
- Preserved every Tool NodeRun as its own WorkGraph node instead of folding failed and successful calls into one aggregate; exact runtime retry identities now add only a retry edge, while an unlinked latest success after a same-owner failure is still promoted so recovery remains visible without inventing a relationship.
- Kept the desktop Header WorkGraph entry permanently visible and pinned outside overflow menus; opening it now retains the session's latest completed managed graph until a newer managed WorkGraph is created, while later runtime-only conversation rounds remain excluded from the canvas.
- Clarified orchestration guidance so independent managed parallel Work Items go to different Room Agents, while one Agent uses Subagents inside a single owned responsibility; assigning sibling Work Items to the same Agent is now described truthfully as a serial queue instead of parallel execution.
- Matched Composer WorkGraph Dock Agent avatars to the chat message avatar's 32-pixel footprint and tightened the surrounding controls and spacing without changing full-graph node sizing.
- Kept terminal root Agent Attempts visible in bounded Execution snapshots independently from newer child Subagent Attempts, so `submit_work` can record its Submission and the Room slot can settle idempotently after subagent-backed work.
- Completed the WorkGraph's read-only whiteboard navigation with blank/Space/middle/right-button panning, two-dimensional trackpad scrolling, pointer-anchored wheel and pinch zoom, blank double-click zoom, keyboard pan/zoom/fit controls, resize-aware symmetric travel space, stable focus centering, screen-size detail inspectors, and blank-click or Escape dismissal without stealing graph interactions.
- Removed the redundant rounded background and border from the WorkGraph's internal layout canvas so the whiteboard grid remains the single main surface while only real ownership subgraphs retain visible frames.
- Moved WorkGraph node and edge inspectors onto the high-opacity popover reading surface so board grids, nodes, and edges no longer bleed through long-form detail text.
- Replaced uniform WorkGraph wrench nodes with distinct icons for search, web fetch, local inspection, browser control, terminal/code execution, file mutation, messaging, generation, workflow control, and external capabilities while keeping lifecycle color semantics unchanged.
- Replaced WorkGraph Bézier fans with orthogonal polylines, strengthened neutral forward edges, and muted loop-back saturation; return routing now treats node intersection as the hard conflict while allowing same-side returns to merge after their short source stems onto one shared U-shaped bus and arrow port inside the owning subgraph, with a stable inset safety gap from the rounded frame, reducing visual line count without covering nodes. Sibling retries share lower flow rails, while Subagent ownership stays inside one primary runtime frame without nested containers.
- Preserved every invoked sibling Subagent as its own WorkGraph node, recovered exact child Tool lifecycle from structured Agent results or bounded read-only transcript history, and gave each Subagent a bounded set of representative activity slots: failures and other structurally significant actions take priority, then the latest successful supporting actions fill the remaining slots so recovery stays visible without flooding the canvas. Progress-only facets remain filtered, while one owner-scoped visibility policy surfaces user-observable research, mutation, execution, browser, Artifact, failure, and control actions and keeps older noisy supporting reads plus duplicate domain mutations in detail.
- Anchored WorkGraph review dispatch and changes-requested returns to the exact successful `submit_work` ToolRun that caused the transition instead of drawing the control loop against the whole Agent node.
- Reoriented the full WorkGraph around a top-to-bottom primary responsibility spine, with each Subagent reserving its own downward subtree lane, exact descendants staying aligned beneath their real owner, owner-plus-direct-child scope frames, and same-level Tool or Subagent siblings added from left to right.
- Restored K3-compatible WorkGraph authoring through the proven scalar `plan_document` transport and exact sealed-proposal commit, removing the failed wide incremental-item protocol; substantial tasks now prompt every Agent to assess useful Subagent decomposition, while Room coordinators create a managed graph only when responsibility or topology must persist and never replace structured Assignment with raw `@` dispatch.
- Kept exact Goal-bound Room workers and reviewers in their scoped capability lanes after adaptive promotion instead of rejecting their next wake as a stale coordinator binding.
- Kept the WorkGraph entry available on both desktop and mobile so users can open the shared empty state at any time; the Composer activity Dock still appears only after `plan_execution` has atomically persisted an active Plan with non-empty Work Items, and ordinary runtime-only Agent or Tool observations never become WorkGraph canvas content.
- Made WorkGraph completeness explicit and recovery-safe: runtime projection now keeps the root and latest runs in a dedicated 256-node/512-edge UI window, reports partial totals instead of silently dropping current work, and persists exact Tool Artifact references independently of message/NodeRun arrival order.
- Kept the last successful WebSocket-refreshed graph visibly marked as stale after a read failure, and made typed edges reliably mouse/keyboard-accessible and inspectable with source/target nodes, exact runtime identities, observed timestamps, and autonomy-preserving retry/control-return explanations; completed retry targets now remain visible from structural edge facts instead of a hard-coded tool-name list, and Gate nodes retain their distinct visual identity.
- Fed bounded, actor-scoped observed Tool/Subagent/Gate outcomes, errors, Artifacts, and exact control edges back into dynamic Agent context after resume or compaction without exposing another Agent's intermediate output or prescribing a route, retry, or next action.
- Made runtime Graph projection converge under duplicate and out-of-order lifecycle events and Provider disconnects without downgrading terminal runs, replaying successful tools, or dropping completed siblings and Artifact evidence.
- Replaced the 1.5-second conversation-wide WorkGraph polling loop and message/round/Goal refresh guesses with owner-and-session-scoped `execution_invalidated` events emitted after durable Plan, responsibility, runtime-node, Goal-binding, and reconciliation mutations; Web and desktop clients debounce exact invalidations, refresh on visibility, and retain a 30-second active-execution disconnect fallback.
- Distinguished completed tool transport from rejected business mutations across DM, Room, Goal continuation, and WorkGraph: rejected calls now stay visible as recoverable failures with concise reason summaries and control-return edges instead of green success cards or raw JSON.
- Preserved every visible Room user message while root messages are repartitioned into Agent-round nodes, and coalesced optimistic/canonical root identities without allowing a later map write to replace unrelated follow-up input.
- Persisted bounded, redacted Tool/Subagent result and error summaries with duration evidence, suppressed internal runtime control sentinels, and rendered failed control returns plus only explicitly correlated Agent-chosen retries as distinct WorkGraph back edges.
- Kept the Room creator/Lead visible as the stable coordinator root and ordered the conversation Agent rail from the same responsibility projection.
- Scoped Room round identity uniqueness to root Attempts, allowing managed Subagents to share their parent Agent's physical round while remaining independently bound by `parent_attempt_id + tool_use_id`; this prevents Subagent admission and final Work submission from failing with an `execution_attempts` unique-key conflict.
- Allowed native Subagents to run as runtime-only graph nodes when no exact managed Assignment binding exists, while preserving strict identity and state checks for durable child Attempts; WorkGraph nodes now show each Subagent's own stable avatar instead of the parent Agent identity.
- Started SQLite transactions with an immediate writer reservation and retried transient admission conflicts from a fresh authoritative snapshot, so an authorized native Agent child starts instead of disappearing behind a generic PreToolUse failure; unexpected persistence failures now retain their internal diagnostic cause.
- Kept every visible Tool and Subagent connected to its owning Agent, and grouped runtime children into compact nested subgraphs without disturbing the main responsibility chain.
- Kept every active Room Agent's exact stop control available after its pending placeholder is replaced, added correlated interrupt acknowledgements and stopping feedback, and terminalized unresolved Feed, Thread, tool, and WorkGraph activity after interruption.
- Decoded Windows sidecar stdout and stderr as UTF-8 so startup diagnostics preserve readable structured logs.
- Relocated the complete macOS or Windows desktop state root after confirmation, then restarted directly and safely rebased stored paths with automatic rollback on startup failure.
- Removed the unsafe host-workspace HTTP setting; server deployments now configure their workspace root only through the deployment environment.
- Avoided duplicating a durable Agent failure as a second conversation system-error bubble.
- Kept runtime stream diagnostics in structured logs while showing concise recovery guidance in DM and Room conversations.
- Preserved chronological DM history when newer durable round indexes are merged with older runtime transcripts after a responsive remount.
- Standardized the Chinese Subagent empty state on the same product terminology as its panel and navigation label.
- Showed the inherited default model state on contact cards instead of a placeholder Provider value.
- Presented contact-card permission modes with localized product labels instead of runtime protocol values.
- Localized contact-card metadata labels consistently across Chinese and English interfaces.
- Removed redundant Skill detail badges when category and source resolve to the same label.
- Localized work-loop trigger types and usage counters instead of exposing protocol values and fixed English units.
- Associated Git Skill import inputs with their visible labels for reliable screen-reader navigation.
- Displayed one-time scheduled-task dates in an unambiguous year-first order.
- Localized scheduled-task templates, creation controls, pickers, and validation feedback in English mode.
- Used platform-appropriate workspace path examples in desktop settings.
- Localized Launcher input and recent-chat accessibility labels across Chinese and English interfaces.
- Localized message actions, activity states, attachments, and generated-file labels across Chinese and English interfaces.
- Localized conversation process summaries, tool-run states, and cache statistics across Chinese and English interfaces.
- Localized tool execution cards, permission details, and result actions across Chinese and English interfaces.
- Localized conversation rulers, round previews, and view-switcher accessibility labels across Chinese and English interfaces.
- Formatted chat-sidebar activity times in the active interface language.
- Localized Room history activity times and row actions across Chinese and English interfaces.
- Localized conversation return-to-latest and unread-jump controls across Chinese and English interfaces.
- Localized Room auxiliary-panel navigation, upload, and resize accessibility labels.
- Localized the earlier-message loading state across Chinese and English conversations.
- Localized the workspace empty-preview title and guidance across Chinese and English interfaces.
- Localized workspace file reveal, focus, edit, save, and sync controls across desktop and web interfaces.
- Localized Agent contact records, message states, reply routes, and activity times across Chinese and English interfaces.
- Localized Skill catalog categories, update states, and import or removal feedback while preserving user-defined category names.
- Localized Skill detail navigation, metadata, availability controls, and action states without translating Skill-authored content.
- Localized the default close-button accessibility label for shared dialogs.
- Localized both Skill import modes, their authoring guidance, examples, controls, and downloaded Room guide.
- Replaced host-language search clear controls with an app-localized clear action across shared search fields.
- Localized community Skill results, previews, source filters, source management, and action feedback while preserving third-party content verbatim.
- Forwarded required accessible names through every Liquid Glass switch so assistive technology identifies the actual setting being toggled.
- Localized code-block actions and Mermaid loading, preview, source, error, and accessibility chrome across Chinese and English interfaces.
- Made narrow conversation workspaces responsive: the Composer hides its decorative `Powered by Nexus` label, while WorkGraph overlays and node spacing follow the local chat width instead of the full window.
- Presented the expanded WorkGraph on a theme-aware low-contrast grid board so responsibility chains and nested Agent execution subgraphs keep their spatial context without competing with node status or direction edges.
- Let a Room coordinator continue the managed WorkGraph immediately after accepting a selected review: the same physical round now upgrades from either a cross-Agent ReviewBinding or a self-review WorkBinding to coordination, exposes newly Ready work, and can create the next directed Assignment without asking the user to send “continue” between nodes.
- Replaced fragile WorkGraph arguments with a durable two-step Plan protocol: `prepare_plan_execution` strictly parses one complete Nexus Plan Document v1 YAML string and seals an immutable, non-authoritative proposal even in Plan Mode; `plan_execution` accepts only its proposal ID and full-fence digest, then materializes the complete graph atomically and idempotently. The digest now seals exact Execution/version/base Plan authority plus Goal activation, reserved successor, and predecessor; commit re-resolves trusted Goal state so an ambient Goal cannot appear, disappear, or retarget after preparation.
- Made Plan proposal recovery attributable and concurrency-safe: only the exact authoritative Plan event written by the proposal's stable command counts as its materialization receipt, while semantic graph equality alone does not; a CAS claim lease serializes foreground replay and background reconciliation, initial reservation replays cannot inherit the winner's lease, and a racing `blocked` row automatically converges only when that exact receipt exists. Recovery also supports the first Plan on an existing planless Execution; Goal resolution now requires exact non-empty owner metadata, and a missing Goal confirmer keeps the durable proposal at `confirmation=pending` so completion and continuation remain fail closed until confirmation succeeds.
- Removed duplicate full Execution Snapshots from MCP results, keeping only revision, recovery actions and authoritative actor-specific context; Goal completion-tool fallback can no longer bypass the current-round Objective Alignment gate.
- Removed full-screen blank gaps inside active Room Agent cards by scoping streaming height stabilization to the current Assistant turn, so tool continuations reset stale text height without reintroducing streaming jitter.
- Restored canonical Room Agent reply order when live child events arrive before the initial history snapshot: durable `display_order` now backfills earlier executions without letting legacy timestamps or later volatile evidence reshuffle visible cards.
- Reworked the Room unread boundary from a floating button-like badge into a centered horizontal reading divider, moved unread jumps to a contextual reading position, and made long virtualized conversations refine their initial index jump against the mounted message instead of stopping short of the real unread target.
- Preserved explicit ordinary-Agent model selections when their Provider is temporarily unavailable: runtime now falls back to the user default and restores the original selection automatically when it becomes usable again, while the Nexus main Agent always follows the default model.
- Aligned capability detail navigation with the shared workspace header and native macOS window controls.
- Added a subtle motion-safe text flow across all message activity indicators.
- Repaired the migration 61 collision between private Skill sources and Execution orchestration, preserving existing private-source databases while applying the complete WorkGraph schema in order.

### Changed

- Replaced full-width Agent launch tool cards with equal-width task entries that use stable Skill-style subagent avatars, truncate long titles, keep status at the trailing edge, omit redundant live-tool text, and open the exact subagent thread in the shared desktop or mobile panel.
- Reduced the conversation task summary to a compact flat status anchor with a responsive detail popover.
- Reused Provider brand icons in the first-run model setup and prioritized domestic API-key services in its catalog.
- Added WorkGraph Node Run history with bounded result/error detail and exact structured Artifact links, plus local-only collapse, full-text search, zoom, fit, and current-node navigation for large DM and Room graphs.
- Added persistent Room member pause and resume controls that stop the member's current slots, preserve exact queued work, and gate Agent wake, Goal continuation, and WorkGraph dispatch until participation resumes.
- Added a Room Composer “Stop all” action that freezes the active Agent-round target set at click time and sends one exact acknowledged interrupt per member.
- Simplified the macOS and Windows update-ready prompts and made download progress windows substantially more compact.
- Added one authoritative managed WorkGraph resource with two deliberately different views: a Composer Agent activity Dock for exact output jumps and an icon-first Header surface for inspecting the complete graph. The Dock contains only deduplicated primary Agents and uses green exclusively for live Agent work; Work Item responsibilities, nested Subagent identities, Tool/Gate nodes, typed direction edges, and exact Agent-round Task steps remain in the full graph. Runtime-only DM and single-Agent activity remains internal diagnostic evidence and is never returned as public WorkGraph canvas content.
- Removed the duplicate expandable graph above the Composer and added a node-adjacent floating inspector for objectives, deliverables, acceptance criteria, blockers, submissions, reviews, bounded result/error summaries, duration, exact retries, node-local Tasks, and direct Tool/Subagent activity.
- Added the built-in `execution-orchestrator` Skill and reduced the always-on orchestration prompt to stable capability boundaries: Agents independently evaluate local-state pressure, context-fork value, responsibility boundaries, topology value, persistent ownership, evidence branches, and continuity risk, then choose the minimum sufficient combination of direct work, Task, subagent, Work Item, WorkGraph, Gate/Loop, Room, and Goal. The backend keeps only identity, authorization, exact binding, idempotency, declared dependency/scope, immutable history, and state-consistency constraints; acceptance criteria, output scopes, terminal markers, reviewer separation, and Goal-promotion signals are no longer workflow admission requirements.
- Split execution orchestration and Goal lifecycle guidance into progressively loaded Skill references, reduced the always-on orchestration prompt to stable boundaries, and kept MCP descriptions focused on atomic state contracts instead of repeating Room strategy, graph tutorials, use-case routing, or final-response policy.
- Added a shared Objective Alignment contract, managed Goal audit, and optional Execution Gate: an Agent can record criterion-level `aligned`, `not_aligned`, or `inconclusive` evidence in the runtime graph without starting a Goal or letting the backend choose the next route; only an actually chosen rerun creates a new Node Run.
- Added a deterministic actor-scoped graph digest to model Execution context: coordinators see the current DAG, members see only their responsibility slice and accepted upstream topology, and Mermaid remains a derived UI/debug view rather than an execution protocol.
- Made ordinary Plan growth a monotonic successor revision: existing nodes and incoming dependencies must remain unchanged, new edges may target only new nodes, omissions or rewrites require explicit supersede authority, and activation waits for a quiescent responsibility boundary. Explicit Goal creation and evidence-backed adaptive Execution promotion now have separate model rules.
- Aligned Slash command descriptions on a reserved command-name column while keeping argument hints anchored to the right edge.
- Replaced the duplicate runtime version card with the desktop app's authoritative version, build number, and log export action in General settings.
- Bounded every model-facing Execution contract collection to 32 items across MCP schema, Plan Mode, service normalization, and storage commands with typed overflow rejection; abnormal legacy context now marks truncation explicitly while WorkBinding/Dispatch fail closed, and file/directory output conflicts now use conservative Unicode NFC plus case-fold comparison without changing persisted display scopes.
- Added explicit transient Execution replacement and abandonment boundaries: coordinators can replace a non-Goal objective only by atomically submitting its complete successor Plan, or cancel it without creating a successor; same-objective route changes remain replans, accepted work never carries automatically, and Goal-bound requests return a typed retarget requirement.
- Unified Goal, Plan, Work Item, Assignment, subagent, and Room execution around one durable WorkGraph: Agents now receive state-derived actions, structured handoffs and trusted WorkBinding fences, explicit and adaptive Goals bind the same Execution, blocked work can resume with evidence, replans atomically supersede live responsibility chains, and duplicate production or Room wakeups are rejected by backend state. Every transition that interrupts live work now captures an exact Room slot or runtime round in a same-transaction durable cancellation outbox with lease recovery and retry, so delayed cleanup cannot stop successor work and missing physical identity is reported instead of guessed; provider-accepted interrupts are distinguished from Nexus-local round cancellation when a shared session makes provider interruption unsafe. Goal objective changes now advance through a recoverable revision rebase that terminalizes the old graph, reserves exactly one successor Execution, and blocks completion until its first Plan is durably bound. Cross-Agent Room reviews return through an independent durable review outbox and trusted ReviewBinding, while self-review remains inside the current WorkBinding and does not manufacture a second reviewer round.
- Made subagent parent-round exit recovery durable with a cross-layer fixed grace deadline and a Room-independent startup/periodic reconciler, so a missing terminal SDK event cannot leave child work running after a restart; internal Goal continuation now requires an exact Goal/revision/Execution binding before claim and runtime launch; permanent Room Dispatch failures now reopen work only for the exact current unstarted responsibility, while stale graphs and real running Attempts remain untouched.
- Made semantic bold text clearly distinguishable in Chinese Markdown while preserving the regular prose weight.
- Reduced the Composer context-usage ring while preserving its interaction target and footer spacing.
- Kept Room headers fully navigable at medium widths, with progressively compact view and member controls, click-to-toggle active panels, a tighter trailing gutter, and no redundant guide menu.
- Made the desktop-only sidebar update action start the native macOS and Windows download, verification, and installation flow instead of opening the GitHub release page.
- Kept permission confirmations and user questions pending until answered or explicitly cancelled, including across WebSocket reconnects; paused the round idle watchdog while waiting and projected the needs-response state into the chat sidebar.

## [0.1.34] - 2026-08-05

### Added

- Added native folder pickers to the desktop data-directory setting on macOS and Windows.
- Added separately signed and notarized macOS installers for Apple Silicon and Intel, with architecture-aware automatic updates.
- Added read-only CC Switch discovery and idempotent Provider/model import across onboarding and settings, with runtime compatibility guidance and default model setup.

### Changed

- Unified Composer send, stop, permission, question, status, and context surfaces into stable compact controls that preserve layout across pending and completed states.
- Moved the desktop update shortcut into the sidebar footer and aligned macOS management headers, wordmark, sidebar toggle, and Dock axes with native window controls.

### Fixed

- Kept Agent configuration dialogs stable across tabs, preserved structured permission timeout reasons, and prevented malformed persisted tasks from crashing conversations.
- Hardened direct desktop upgrades across quoted owner IDs, regenerable caches, historical summary ACLs, and launcher-owned Linux permissions without recursive mode rewrites.
- Treated AutoDream Agents without an available provider and model as a deferred check instead of repeatedly logging runtime errors.
- Made DM acceptance durable across slow runtime startup, WebSocket disconnects, and lost-ACK recovery without discarding uncertain messages.
- Routed main Agent creation through versioned workspace initialization and made platform, host, owner, and Agent Skill discovery canonical, bounded, live-updating, and consistent with runtime projections.

## [0.1.33] - 2026-08-04

### Changed

- Streamlined first-run Provider setup with direct model selection, a docked action bar for long catalogs, and in-dialog custom LLM connections.
- Changed the default permission for new preferences and Agents to automatically accept file edits while retaining approval rules for other actions.

### Fixed

- Prevented persisted TodoWrite plans that use legacy task fields from crashing the conversation task panel.
- Kept Agent Skill cards readable when the conversation detail panel is resized by adapting the grid to the panel's actual width and clamping long titles to two lines.
- Repaired existing isolated Agent workspaces and prevented unreadable subtrees or subscription failures from breaking file access or appearing as conversation errors.
- Prevented Windows upgrades from failing with access denied when Nexus was still running in the system tray.
- Repaired direct upgrades from v0.1.27 and v0.1.28 so Agent workspaces, transcripts, and Room files migrate safely without changing launcher-owned permissions or causing isolated Linux restart loops.

## [0.1.32] - 2026-08-03

### Added

- Added a conversational first-run Nexus experience that guides users through model setup, connection verification, and a concise product introduction before their first task.

### Changed

- Localized the remaining conversation, Room, workspace, Agent, Skill, automation, Launcher, Markdown, and accessibility controls across Chinese and English interfaces while preserving user-authored and third-party content.
- Simplified the macOS and Windows update-ready prompts and made download progress windows substantially more compact.

### Fixed

- Decoded Windows sidecar stdout and stderr as UTF-8 so startup diagnostics preserve readable structured logs.
- Relocated the complete desktop state root through an offline restart-safe migration with stored-path rebasing and rollback, while keeping server workspace roots environment-controlled.
- Kept runtime diagnostics in structured logs, showed concise recovery guidance, and prevented durable Agent failures from producing duplicate conversation errors.
- Preserved chronological DM history when newer durable round indexes are merged with older runtime transcripts after a responsive remount.
- Corrected inherited model and permission labels, scheduled dates, platform-native workspace paths, Skill metadata duplication, and assistive names across shared controls.

## [0.1.31] - 2026-08-01

### Added

- Added the MIT-licensed `diagram-design` and `Kami` built-in Skills for editorial diagrams, documents, slides, and landing pages, with pinned sources and explicit third-party provenance.
- Added safe Agent Memory browsing, editing, and confirmed deletion for topic and daily-log documents while protecting the root `MEMORY.md` index.
- Added per-Session model and permission overrides, authoritative context-window usage, native memory-recall indicators, and per-Agent Room projections.

### Changed

- Reworked Agent Tools, Skills, Contact, and Memory into compact responsive management surfaces with guarded auto-save, clearer permission guidance, and one consistent reading plane.
- Unified installed, update, community, and Agent Skill cards with localized descriptions, deterministic mathematical avatars, responsive catalogs, and quieter status presentation.
- Simplified Room threads, workspace panels, file navigation, resize gutters, member selection, and management-page headers while keeping final replies in the main conversation feed.
- Moved model and permission selection into per-Session Composer controls with stable multi-Agent cascading, explicit reset actions, and clearer approval and full-access language.
- Replaced the startup animation with a lightweight local indicator, suspended idle Home animation work, and smoothed visible workbench motion.
- Made General settings show the authoritative desktop version, build number, and log export action instead of a duplicate runtime version.

### Fixed

- Fixed runtime startup, replacement, interruption, reconnect, and live configuration races so stale processes or delayed results cannot affect a later turn.
- Kept internal interrupt markers out of conversations, permission results, persisted history, and diagnostic logs.
- Closed application, CLI, handler, logging, Session, Agent, Room, and migration resources consistently, preventing Windows file and SQLite lock leaks.
- Hardened Windows runtime paths, shell authorization, read-only bundled Skills, file permissions, and cross-platform tests.
- Restored context-window snapshots after refresh or restart, decoded structured Session keys, and stabilized anchored overlays and action menus.
- Restored macOS packaging on Node installations without Corepack and kept compact headers clear of native window controls.

## [0.1.30] - 2026-07-31

### Added

- Added a complete Slash command workflow across DM and Room Composer, including autocomplete, `/model`, `/skills`, runtime-owned `/compact`, and native Skill dispatch.
- Added exact Room unread navigation plus persistent conversation tabs, per-session drafts, input history, and safe batch history cleanup.
- Added bundled slide-making and WeChat article-search Skills, alongside a unified Skill inventory with per-Agent enable controls.
- Added guided Provider setup and official QR-based connection flows for Feishu, DingTalk, WeCom, and Feishu Docs.

### Changed

- Unified DM and Room interaction handling in the Composer, with one ordered queue for permissions, questions, plans, Goals, Tasks, and return-to-latest controls.
- Refined multi-Agent Room collaboration with caller-scoped Subagent views, stable handoffs, controlled fanout, and per-Agent progress inspection.
- Reworked conversation rendering, scrolling, message ordering, attachments, and responsive surfaces for more stable long-running sessions.
- Improved Windows and macOS update checks, availability indicators, installer download progress, and desktop interaction details.
- Strengthened owner-scoped state, workspace, memory, attachment, and runtime isolation while improving Docker build and runtime cache behavior.

### Fixed

- Fixed Slash command, model switch, Skill execution, hidden-context clearing, transcript restoration, and Claude headless runtime behavior.
- Fixed DM and Room streaming, unread boundaries, Agent arrival order, WebSocket bindings, approval routing, drafts, history reloads, and scroll ownership.
- Fixed Goal lifecycle and token accounting across parent/child Agents, retries, handoffs, restarts, and incomplete runtime evidence.
- Fixed SQLite migrations, orphaned data, legacy layout upgrades, runtime ACL repair, Linux launcher permissions, Windows migration builds, and Docker startup failures.
- Fixed desktop OAuth callbacks, Feishu authorization stages, Windows update builds, Safari rendering, compact Launcher hit testing, and stale editor saves.

## [0.1.29] - 2026-07-26

### Added

- Added opt-in Linux runtime isolation for nxs and Claude Code with stable per-owner identities, shared-project ACLs, a trusted launcher, environment scrubbing, mandatory path policy, and Landlock enforcement.
- Added owner-scoped shared-project management across the API and Operations UI, with confined host file access and immediate runtime recycling when memberships change.
- Added opt-in Linux cgroup v2 process-tree reaping so revoked or closed owner runtimes can be terminated as one trusted group.

### Changed

- Moved persistent host state under `.nexus/app` and owner workspaces and runtime configuration under `.nexus/users/<owner>/`, with an idempotent migration from the legacy layout.
- Rebuilt the desktop shell around one responsive navigation system, pinned the Nexus main agent as an undeletable DM, and made chat, Agent management, and capabilities first-class directories across desktop and phone layouts.
- Added a native Windows app bar with working navigation, menus, caption controls, resize and drag behavior while keeping WebView content fully interactive; restored the macOS Home drag surface without affecting browser layouts.
- Unified Windows menus, update prompts, startup errors, application dialogs, overlays, tabs, search fields, and selection states behind the shared Nexus design tokens.
- Reworked the conversation Composer, Session history, Room tabs, task details, message spacing, and wide-screen conversation rail for clearer focus and responsive navigation.
- Unified Agent and Room creation and editing, including backend-provided Agent templates persisted as workspace `AGENTS.md`, responsive avatar and model controls, and mobile member management.
- Standardized typography, spacing, radii, theme colors, shadows, and semantic font sizes across the application while removing obsolete style and localization definitions.
- Required authenticated local Skill imports to use uploaded archives instead of arbitrary host filesystem paths.
- Updated the bundled nxs runtime channel to `nxs-v0.1.16` while retaining the unchanged bridge dependency at `v0.1.21`.

### Fixed

- Hardened legacy state migration against transient missing files, Finder metadata conflicts, Room overlay subset merges, and misplaced completion markers.
- Fixed unauthenticated App/Web requests resolving to unscoped Agent or automation data instead of the single-user system owner.
- Isolated DM and Room input-queue replay by execution scope and prevented Room recovery from dispatching DM work through the wrong runtime.
- Bound Room stop actions to the exact Agent round so interruption produces one monotonic stopped state without duplicate empty messages.
- Preserved Claude Code subagent transcript projection through confined symlink reads and restored session metadata writes before an Agent workspace exists.
- Restored Windows title-bar, menu, WebView, resize, caption, horizontal Session scrolling, and startup-log behavior across mouse, touchpad, and constrained layouts.
- Fixed desktop release packaging so the bundled nxs runtime keeps its required ripgrep sidecar on macOS and Windows.
- Fixed desktop local profiles receiving an implicit server subscription and incorrectly exposing or enforcing account quota.
- Added syntax highlighting for recognized workspace source files and kept short Markdown previews top-aligned.
- Corrected Tailwind semantic font generation, class merging, theme tokens, typography weights, and primary-color rendering that had inflated or dropped styles.
- Fixed responsive Session, Room, sidebar, Agent editor, workspace header, conversation switcher, composer, scroll-control, and phone-layout geometry.

## [0.1.28] - 2026-07-23

### Fixed

- Aligned nxs and Claude Code message projection across effective result errors, empty assistant suppression, streamed tool input, nested tool ancestry, throttled shell progress, and forward-compatible content blocks so malformed or newer runtime output cannot silently end a conversation.
- Fixed imported transcripts exposing SDK output-limit recovery prompts as repeated user messages and generating empty interrupted assistant bubbles in the conversation timeline.
- Fixed Room Skills failing before runtime startup when legacy or imported skills did not define the removed `runtime_instructions` field; Room now injects each selected Skill's frontmatter-stripped body directly.
- Fixed newly created custom Providers defaulting to the Anthropic Messages API format instead of the first format listed in the selector.
- Fixed incomplete provider tool JSON terminating a DM round; nxs now returns a recoverable tool_result, lets the model retry, and keeps that internal recovery out of the user-facing timeline. Genuine runtime errors are carried by the terminal round status and restored from the durable result summary, so the frontend still shows the cause after reconnecting.
- Fixed runtime switches failing when cleanup of the previous Claude Code or nxs process returned a stale transport error, and made generic startup guidance runtime-neutral.
- Fixed explicit Claude Code/nxs selections being overridden by a stale process-level runtime environment, keeping provider credentials and runtime-specific settings aligned with the selected runtime.
- Fixed runtime-scoped compaction settings so Claude Code receives its native auto-compaction threshold and model context cap, while nxs keeps Nexus-native environment keys.
- Fixed the conversation Agent surface disappearing while context compaction is visible; the live message now keeps the Agent identity and shows the compaction activity state.
- Fixed the desktop provider scope recovery skipping ownerless public providers created after the 00018 migration (they were mislabeled as intentional subscriptions and became uneditable), and added a last-resort pass that assigns providers referenced by no runtime or preferences to the local principal and owner users.
- Made the macOS desktop smoke test wait for each requested launcher navigation to finish and become ready before continuing, preventing overlapping WebView loads from racing the exit command.
- Kept subscription quota enforcement on internal Goal continuations and now project exhausted account quota as an actionable `usage_limited` Goal state instead of a generic continuation failure.
- Fixed the Windows desktop release-notes build by explicitly selecting WPF alignment, font, color, and brush types.
- Bound WebSearch API keys to their selected provider so a key from one provider is never displayed or reused under another provider.
- Fixed desktop updates retaining old downloaded app and installer packages in `~/.nexus/cache/updates` after a newer version started successfully; deferred downloads remain available until then.
- Fixed macOS and Windows update dialogs allowing long release notes to push action buttons out of view; release notes now stay in a bounded scrollable container with Markdown formatting.
- Rebuilt the launcher hero as a fixed-size stage with a single uniform scale factor, replacing the per-breakpoint transform patches; anchored the decorative agent pile to the viewport bottom so short windows keep a full-size cloud, and aligned the pile physics world with its container width so tokens spread correctly.
- Fixed conversation auto-follow losing the bottom position when the chat viewport resizes (small app windows, growing composer) and after the feed switches between static and virtualized rendering.
- Fixed Room @mentions that were routed successfully but rendered as plain text, and accepted Unicode punctuation around parenthesized Agent IDs so public handoffs continue reliably.
- Sorted built-in Provider entries by English display name in the settings sidebar.
- Fixed Provider model tests for full operation URLs and query-bearing Azure endpoints, normalized Azure resource/project roots to `/openai/v1/responses`, added Azure `api-key` authentication across model tests and lightweight backend requests, enforced `store=false` and the Responses minimum `max_output_tokens` probe value, and return an actionable error when an Azure deployment, image, or Chat Completions operation URL is selected for Responses.
- Switched Azure OpenAI Chat Completions model tests and lightweight backend requests from `max_tokens` to `max_completion_tokens` for compatibility with newer deployments.

### Changed

- Updated the SDK bridge dependency to `v0.1.21` and the bundled nxs runtime channel to `nxs-v0.1.15`.
- Unified platform-owned Skills behind one global compatibility root for nxs and Claude Code; Agent records now persist selected platform `skill_ids` instead of copying platform Skill files into every workspace.
- Unified imported third-party Skills behind the owner workspace source `<workspace>/<owner>/.agents/skills`, shared by nxs and Claude Code; Agent records now persist `external:<skill_name>` references, with a one-time migration preserving v0.1.27 registry data and Agent installations.
- Realigned light-theme inputs, hover feedback, sidebar borders, and conversation-tab dividers with the restored cool-gray page background.
- Unified control, card, overlay, and content radii around a restrained shared scale.
- Replaced the full Room history side panel with an anchored dropdown that shows ten conversations per page while retaining rename and delete actions.
- Made conversation tabs responsive to available header width, showing recent titles only and loading conversation content on selection.
- Hid the AGENTS.md profile editor for the main Nexus agent, which intentionally has no workspace AGENTS.md.
- Split Room runtime append prompts into stable and dynamic cache segments, reused warm Room slot runtimes without replaying the full public context, and kept the legacy flattened prompt for runtime compatibility.
- Unified sidebar conversation activity around Room IDs so DM and group rows share one transient execution source, removed Agent runtime status subscriptions from chat and contacts navigation, and dropped the unused directory-side runtime projection.
- Removed the unused Agent runtime status HTTP endpoint and the legacy runtime-only workspace subscription mode.

### Added

- Added the bundled `ima-skill` 1.1.8 package to the platform Skill catalog.
- Added debug-only prompt-cache segment diagnostics with safe per-segment hashes, sizes, roles, and cache-control metadata.
- Added a textured Nexus mascot avatar, random avatar assignment for new Agents, and stable avatar fallbacks for existing records without an avatar.
- Added OpenAI Responses as an `nxs` Agent runtime protocol, including runtime-specific Provider selection, explicit protocol and multimodal environment projection, auxiliary vision routing, and safe startup diagnostics.
- Added an opt-in process integration test that proves Nexus runtime configuration reaches a real nxs child and requests `/v1/responses` through the bridge.
- Added explicit nxs passthrough for OpenAI prompt-cache enablement, mode, TTL, and legacy retention controls.
- Added a built-in Azure OpenAI provider with resource-level v1 endpoint normalization, Chat Completions and Responses formats, and explicit deployment-name model configuration.

## [0.1.27] - 2026-07-19

### Added

- Added runtime-scoped ToolSearch settings, provider-configurable WebSearch, and an independent visual-model route for nxs conversations.
- Added durable Room delayed wakes, causal wake metadata, bounded per-Agent queues, and scheduler leases, jitter, misfire handling, limits, and expiration.
- Added per-Agent nxs settings projection, host-coordinated AutoDream maintenance, a file-backed Memory view, and a capability-driven subagent inspector.
- Added signed and notarized macOS packaging, release metadata validation, and desktop update/cache recovery support.

### Changed

- Refined onboarding, workbench, navigation, typography, fonts, Markdown, and capability surfaces into a flatter, denser visual system.
- Reorganized frontend ownership around explicit projections and controllers across conversations, Rooms, Agents, settings, skills, channels, previews, and scheduled tasks.
- Made Room context budgets model-window-aware, kept runtimes warm through the shared idle reaper, and reduced communication/tool prompt overhead.
- Consolidated Tool Search and scheduled-task MCP surfaces around intent-level capabilities, with runtime selection and compaction state visible in the Composer.
- Moved long-term memory ownership into the nxs subprocess and added a one-time migration for legacy product-managed memory skills.
- Simplified workspace, Markdown, Office, image, and document-preview pipelines with explicit parsing and presentation boundaries.
- Updated the SDK bridge dependency to `v0.1.20` and the bundled nxs runtime channel to `nxs-v0.1.14`.

### Fixed

- Hardened DM and Room input queues, ACK/retry handling, stop/interrupt delivery, Goal replacement, and durable Agent-to-Agent handoffs across restarts.
- Stabilized Room and Thread timeline ordering, agent-round identity, streaming follow, public replies, mentions, and subagent task projections.
- Rejected stale asynchronous responses across conversations, Rooms, Agents, files, settings, goals, channels, and task controllers.
- Aligned permission modes, model context limits, provider/account quota feedback, Provider scope recovery, and runtime compaction behavior.
- Restored workspace image/artifact links, task history, WebView cache invalidation, desktop window sizing, and missing-asset recovery.
- Fixed the Windows desktop update prompt build by disambiguating WPF and WinForms types.
- Prevented macOS WebView recovery checks from interrupting in-flight navigation and added cancellation-aware startup diagnostics.
- Fixed macOS CI DMG checksum validation to resolve artifacts from the package output directory.
- Hardened macOS desktop smoke shutdown with a diagnostic SIGTERM fallback when the exit notification is not delivered.

## [0.1.26] - 2026-07-08

### Changed

- Reworked the conversation turn protocol: the backend now mints `round_id` / `user_message_id` / `agent_round_id`, the frontend only sends `client_request_id` / `client_message_id`, and `chat_ack` returns the canonical ids. Removed the legacy `req_id == round_id`, `message_id == round_id`, and `round_id:agent_id` suffix conventions (breaking realtime protocol change; old on-disk history is normalized at read time).
- Room agent slots now emit explicit `agent_round_status` lifecycle events, permission requests carry `round_id` / `agent_round_id` / `message_id` / `tool_use_id` for exact binding, and slot interrupts target `agent_round_id`.
- Added a backend `ConversationTurn` projection with new history endpoints (`/sessions/{key}/turns`, `/rooms/{id}/conversations/{id}/turns`, turn index), and unified the frontend DM/Room timeline grouping behind a single projection hook.
- Reduced Agent tool pre-authorization settings to only the tools that benefit from explicit allow rules, while retiring basic, managed, and interaction-only tools from the editor.
- Clarified the default Agent and Nexus prompts so internet research pairs `WebSearch` discovery with `WebFetch` source verification without changing permission defaults.
- Refined empty conversation composer shortcut hints and the desktop send button label.

### Fixed

- Rotated assistant segments by snapshot message id in history projection so multi-segment rounds no longer collapse into one message (which corrupted content and message ordering after a session resync), auto-collapsed thinking/process sections once a round finishes, and stopped duplicating the final answer when a runtime's result summary text differs from the message body.
- Injected macOS desktop window chrome metrics into the Web runtime so top-edge content uses the native drag-strip height as its single source of truth.
- Prevented ad-hoc, non-notarized macOS release packages from being offered as automatic desktop updates.
- Made macOS desktop termination wait for sidecar shutdown and preserve pid records when forced cleanup cannot finish.
- Added Windows desktop sidecar orphan cleanup and a short port-release wait before binding the fixed local port.
- Fixed login recovery when old session cleanup fails, bounded `nxs` runtime release lookup timeouts, restored deleted core tests, and enforced subscription token quota before new DM/Room runtime rounds.
- Updated the Nexus Agent SDK Bridge dependency to `v0.1.18`.

## [0.1.25] - 2026-07-05

### Changed

- Rebuilt desktop releases against the refreshed stable `nxs` runtime channel so packaged apps include `nxs-v0.1.11` with the bundled `rg` sidecar.

## [0.1.24] - 2026-07-05

### Changed

- Streamlined runtime startup success logging, Goal runtime usage test logging, and PNPM command selection.
- Limited the KingHwa font override to chat output so the rest of the UI keeps the standard typography.

### Fixed

- Kept the Agent tool available in runtime allowed-tool lists.
- Propagated submit interrupt reasons through the SDK bridge and classified SDK abort stream closes as intentional interrupts instead of generic runtime failures.

## [0.1.23] - 2026-07-04

### Added

- Added session-scoped provider diagnostics for `nxs` and surfaced background subagent task lifecycle events across indexing, DM, and Room transcripts.
- Added Background Tasks follow-up messaging, conversation session navigation, subscription operations, and Room Goal loop/title improvements.

### Changed

- Refined Skill update discovery, update/import busy states, desktop window chrome, sidebar density, runtime retry copy, and frontend camelCase module boundaries.
- Updated bridge/runtime integration for subagent tasks and provider diagnostics while reducing noisy SDK stderr output.

### Fixed

- Fixed imported Skill update recovery, partial Skill redeploy failure reporting, title generation, room conversation sorting, GLM runtime ToolSearch behavior, and spreadsheet preview dependency regressions.
- Fixed subagent and Goal continuation regressions, Room thread scrolling, WebSocket recovery, compact-boundary visibility, terminal error summaries, and several Room runtime data races.
- Renumbered post-merge sqlite/postgres migrations so versions 44, 45, and 46 apply without duplicate Goose migration versions.

### Security

- Cleared frontend audit findings by overriding vulnerable transitive `js-yaml` and `@babel/core` versions.

## [0.1.22] - 2026-06-22

### Fixed
- Captured sidecar startup failure output so desktop startup failures include the process error details.

## [0.1.21] - 2026-06-18

### Fixed
- Fixed IM group pairing so Feishu, Discord, Telegram, and other threaded group ingress can reuse a group-level approved pairing while still replying to the current platform thread or message.
- Fixed personal WeChat multi-account QR login management so scanned accounts are stored independently, shown in channel setup, removable one by one, and no longer overwrite top-level channel credentials; documented Docker proxy overrides and single-worker IM deployment expectations.
- Disabled the Provider settings toggle for default models and added an explicit reminder before users can try to turn off a model that must stay enabled.
- Defaulted the built-in image generation tool on only when an image-generation Provider is configured, including scheduled-task permission checks, so imagegen skills can call `generate_image`/`edit_image` without enabling the tool for unconfigured workspaces.
- Kept the Provider settings model list constrained to the remaining page height so long model catalogs scroll inside the list container instead of stretching the settings page.
- Made Docker server deployments generate and persist a connector credentials key when missing, validate malformed keys at startup, and pass standard outbound proxy variables so personal WeChat iLink and Feishu OpenAPI/WebSocket requests can use a server-side proxy.
- Exposed runtime endpoint options in the IM channel configuration for DingTalk, WeChat Work, Feishu, Telegram, and Discord, and made Docker/server-side proxy handling apply consistently to IM HTTP and WebSocket clients, including `ws://` and `wss://` long connections.
- Hardened Docker deployment defaults by pinning container-only Nexus runtime paths, isolating Docker database/log/workspace paths from desktop `.env` values, rewriting loopback host proxy URLs to `host.docker.internal`, using the stable bundled `nxs` release channel, and removing the unused 443 port mapping from the default nginx service.
- Fixed Docker web builds by including the markdown spec imported by the frontend build context, and made runtime image `uv` installation more tolerant of slow package mirrors.
- Stopped malformed `CONNECTOR_CREDENTIALS_KEY` values inherited by Docker deployments from causing restart loops; the entrypoint now falls back to the persisted key file or generates a new Docker key.

## [0.1.20] - 2026-06-11

### Added
- Added configurable IM channels for Telegram, Discord, Feishu, DingTalk, and WeChat Work, including DingTalk Stream ingress, WeChat Work intelligent bot long-connection handling, channel routing, and capability page setup guidance.
- Added a separate personal WeChat channel with built-in Tencent iLink QR login, getUpdates polling, sendMessage delivery, typing status, structured ingress, pairings, and session-key documentation.
- Added Feishu reply/thread metadata, typing reaction indicators, and reaction-created ingress handling to better match OpenClaw-style IM behavior.
- Added shared IM channel HTTP/text delivery and typing lifecycle helpers with failure backoff, and filled Discord/Telegram parity details for typing indicators, Telegram topic delivery, and mention-safe Discord replies.
- Added a shared IM message envelope/receipt model, migrated channel delivery to `DeliverMessage` results, captured Telegram/Discord/Feishu/personal WeChat message ids, and surfaced external platform message ids in automation delivery summaries.
- Added a code-backed IM channel capability matrix and persisted inbound IM envelope metadata onto durable DM round history.
- Added durable external IM delivery receipt overlays so DM assistant replies retain outbound channel, target, thread, and platform message ids in normalized history.
- Added a reusable IM inbound migration module and explicit inbound envelopes for Discord, DingTalk, WeChat Work, and personal WeChat callbacks.
- Added IM channel capability chips to the channel directory so users can compare typing, thread, reply, receipt, media, and durable history support per channel.
- Added a channel disconnect action in the IM channel configuration dialog so users can stop a configured bot connection without deleting existing pairings.
- Added manual IM pairing creation from the pairing directory for known external user, group, or thread identifiers.
- Added explicit multi-user IM session coverage so multiple external users can bind to one Agent while each inbound target keeps its own session.
- Added session-scoped IM delivery routes and clearer pairing management so multiple external users under one Agent remain distinguishable by binding key and IM session.
- Added IM-side pairing approval notices so unapproved external users and groups are told to wait for approval in the Nexus pairing console.

### Fixed
- Fixed personal WeChat QR login so multiple scanned WeChat accounts can stay connected under one Agent, with inbound polling and replies routed by account instead of overwriting the previous login.
- Opened the channel capability UI for every ready IM channel instead of keeping Telegram, Discord, DingTalk, and WeChat Work hidden behind a frontend allowlist.
- Deduplicated concurrent DingTalk access-token refreshes and acknowledged Stream callback failures after notifying users through `sessionWebhook`.
- Updated IM channel copy so the iLink channel is displayed as WeChat in the UI and the WeChat Work setup guide follows the Bot ID + Secret intelligent bot flow.
- Unified IM ingress handler responses so every channel returns a successful pairing-required acknowledgement instead of a generic client error when an external target still needs approval.
- Stopped Telegram, Discord, DingTalk Stream, and WeChat polling ingress from sending external failure replies when a message only needs IM pairing approval.
- Switched DingTalk Stream replies to the callback `sessionWebhook` path and made Robot Code optional unless explicit openConversationId group sends are needed.
- Fixed external IM session placement and title generation so IM sessions stay under their Agent session switcher, never use the Agent name as a title fallback, and generate titles through the normal owner-scoped session-only path.
- Fixed a race where generated IM session titles could briefly appear and then be overwritten back to `New Chat` by later DM runtime metadata refreshes.
- Fixed external IM pairing so repeated pending pairings reuse their real id.
- Fixed manual IM pairing creation so re-adding an existing external target updates the existing pairing instead of failing after the upsert.
- Made personal WeChat typing-ticket lookup degrade softly so typing status failures do not affect message polling or reply delivery.
- Standardized the personal WeChat channel identifier on `weixin-personal` and reduced external reply latency by prioritizing final message delivery over post-round bookkeeping.
- Fixed Telegram long polling to subscribe to edited messages so its existing edited-message ingress handler can actually run.
- Fixed Telegram edited messages so edit updates use distinct ingress request ids instead of being deduplicated as the original message.
- Added Telegram polling and inbound diagnostics so Bot API failures and received updates are visible in channel logs.
- Disabled browser autofill on IM channel credential forms so saved login usernames and passwords are not prefilled into bot configuration fields.
- Removed IM channel card status badges so pairing authorization counts are the visible access state.
- Refined IM channel card metadata so handler, bot, and pairing counts are easier to scan.
- Hid IM capability chips from channel cards to keep the channel list focused on pairing access.
- Reordered DingTalk channel credential fields so Client ID and Client Secret appear before optional Robot Code.
- Clarified Discord IM setup copy to distinguish Bot Token from OAuth Client Secret and explain that Application ID is only used for the invite link.
- Migrated the WeChat Work channel configuration to the intelligent bot Bot ID + Secret flow and long-connection `aibot_respond_msg` stream replies.

## [0.1.19] - 2026-06-10

### Changed
- Updated the Nexus Agent SDK Bridge dependency to `v0.1.11` for explicit packaged `nxs` runtime path handling and unified transcript config roots.
- Centralized DM and Room session resume policy so runtime-kind switches reuse compatible transcript history without carrying stale SDK session ids across runtimes.
- Clarified generated workspace guidance and desktop sidecar runtime path propagation around `NEXUS_NXS_COMMAND_PATH`.

### Fixed
- Fixed Windows desktop blank WebView recovery after resume by rebuilding invalid WebView instances.
- Removed stale runtime download/status fallback paths so packaged Nexus hosts rely on their bundled or explicitly configured `nxs` runtime.
- Fixed `nxs` runtime startup context so SDK-side project instruction loading is disabled when Nexus has already injected workspace prompts.

## [0.1.18] - 2026-06-09

### Changed
- Reduced web shell startup preloads by lazy-loading protected app layout/session code and deferring onboarding tour overlay UI until a guide is opened.
- Added `make app-win-run` for local Windows desktop testing and made Makefile Windows app builds bundle `nxs` by default, with `APP_WIN_BUNDLE_NXS_RUNTIME=0` as the opt-out.
- Updated the Nexus Agent SDK Bridge dependency to `v0.1.10` for Windows `nxs` and Claude runtime startup fixes.

### Fixed
- Fixed Windows Agent runtime startup with bundled `nxs`, SDK MCP arg-file materialization, and npm-installed Claude Code shims such as `claude.cmd`.
- Skipped stale SDK session resume when switching Agent runtime kind so `nxs` and Claude do not first try to resume each other's sessions.

## [0.1.17] - 2026-06-08

### Changed
- Defaulted new and unset Agent runtime preferences to `nxs` while keeping explicit Claude overrides available.
- Enabled `nxs` runtime session defaults for cached microcompact, API context cleanup, and Claude Code-style 1h prompt cache TTL.
- Added an opt-in Agent SDK diagnostics setting for `nxs`, surfaced transport diagnostics in Nexus logs, and included runtime debug logs in desktop log exports.
- Updated the Nexus Agent SDK Bridge checksum metadata for `v0.1.8` so release builds work without a local bridge workspace.
- Passed Anthropic-compatible Agent runtime credentials through `ANTHROPIC_API_KEY` for API-backed Agent sessions.
- Updated desktop release packaging to bundle `nxs` from the `nxs-stable` runtime channel instead of pinning an older runtime release.
- Kept Windows Claude runtime launches on the installed Claude CLI shim and added safe DM/Room runtime startup diagnostics for `claude` and `nxs`.
- Kept Anthropic-compatible runtime credentials on `ANTHROPIC_API_KEY` for Claude Code and `nxs` compatibility, with `NEXUS_API_PROVIDER` carrying the provider mode.
- Logged terminal runtime error messages for DM and Room rounds so API/auth failures are visible in desktop diagnostics.
- Refreshed existing GitHub release notes during repeated tag publishing so re-released desktop packages match the current changelog.
- Fixed Anthropic-compatible Agent runtime authentication by routing non-Anthropic provider tokens through `ANTHROPIC_AUTH_TOKEN` instead of `ANTHROPIC_API_KEY`, matching GLM Coding Plan's Claude Code bearer-token setup.
- Restored `NEXUS_NXS_COMMAND_PATH` precedence over packaged `nxs` runtimes so Windows desktop builds can override a bundled runtime with a verified local executable.
- Cleared conflicting inherited Anthropic credential env vars for Agent runtimes so Windows desktop sessions use either bearer-token or API-key auth, not a stale mix of both.

## [0.1.16] - 2026-06-05

### Changed
- Refined Goal creation and status flows with a smaller composer strip, shared edit dialog, required Room Agent ownership, and Codex-aligned add-menu behavior.
- Unified `nxs` runtime discovery around app-root bundled runtimes so Docker and desktop packages use the packaged binary before bridge resolver cache fallback.
- Updated the Nexus Agent SDK Bridge dependency to `v0.1.6` for explicit `nxs` resolver failures and the `nxs-v0.1.2` runtime manifest default.
- Tightened release packaging validation so desktop assets must declare bundled `nxs` runtime metadata and repeated tag builds replace stale app assets.

### Fixed
- Fixed packaged macOS and Windows `nxs` startup by preferring bundled runtimes over stale `NEXUS_NXS_COMMAND_PATH` overrides.
- Fixed native `nxs` support for OpenAI-compatible Chat Completions providers, Settings runtime/model selection, clearer startup errors, and SDK bridge checksum startup.
- Fixed Room conversation runtime cleanup, visible Goal creation progress, macOS updater trust checks, and agent-session tool filtering.

## [0.1.15] - 2026-06-04

### Added
- Added Goal management with the managed `goal-manager` Skill, Codex-aligned Goal MCP tools, app-server HTTP/WebSocket compatibility endpoints, durable continuation recovery, shared Room Goal routing, and runtime status events.
- Added Agent Runtime selection for `nxs`, including `make dev-nxs` and bundled macOS/Windows release runtimes so desktop installs can run without a first-run runtime download.

### Changed
- Aligned Goal semantics with Codex across lifecycle states, budgets, usage accounting, tool schemas/results, plan-mode pauses, hidden continuation prompts, internal context injection, and completion reporting.
- Refined Goal panel behavior with a lighter status strip, clearer create/edit progress, room-specific disabled states, and reduced internal/debug labels.
- Refreshed public and launcher surfaces with restored app entry links, redesigned login visuals, generated mascot assets, and a transparent Launcher send-button mascot.
- Updated desktop packaging, smoke checks, diagnostics, and release workflows to surface bundled runtime metadata and package the matching `nxs` runtime.

### Fixed
- Fixed Goal MCP visibility, managed-tool authorization, runtime client refresh/rebuild, provider/API error surfacing, hidden continuation delivery, pause/interrupt behavior, stale continuation cleanup, and database migration compatibility.
- Fixed Goal usage, wall-clock, continuation progress, retry accounting, Room shared Goal concurrency, and completion finalization so long-running Goals can report usage and stop cleanly.
- Fixed reasoning-capable provider models so their capabilities are passed to Claude-compatible runtimes, enabling `nxs` and Claude Code thinking by default.

## [0.1.14] - 2026-06-03

### Added
- Added macOS desktop self-update installation with release package download, sha256 verification, staged `Nexus.app` replacement, and relaunch through an external installer script.
- Added runtime resilience defaults: idle SDK session recycling and `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE=70` for earlier Claude Code compaction during long workflows.

### Changed
- Refined compact desktop workspace layout, reduced low-signal sidecar logs, and clarified Agent prompts to use `AskUserQuestion` for native confirmations.
- Defaulted new Agents and the main Agent to ask-permission mode without pre-authorized tools.

### Fixed
- Fixed assistant completion and replay consistency across realtime result projection, repeated assistant snapshots, parallel tool actions/results, and transcript history replay.
- Fixed Windows desktop WebView recovery after long idle, window occlusion, restore, or browser process exits by repaint probing and recreating invalid WebView controls.
- Fixed expected stream-closed runtime shutdown handling, Windows `--mcp-config` startup, concurrent managed-Skill workspace preview initialization, and desktop Claude Code command discovery.

## [0.1.13] - 2026-06-02

### Added
- Added a public Nexus landing page at `/` with a real workbench preview, capability storytelling, unauthenticated entry links, and an ICP filing footer link for deployment compliance.
- Added the built-in `nexus_imagegen` runtime tool so Agents can generate and edit images through the configured image Provider without going through the CLI Skill path.
- Added a built-in Doubao provider with Volcengine Ark text and Seedream image-generation branches.

### Changed
- Moved the authenticated Launcher route from `/` to `/launcher`, refined public landing actions, and updated desktop launcher routes so packaged apps still open the authenticated launcher.
- Changed Agent identity to be anchored on `agent_id`; Agent names are now reusable display labels during creation and rename.
- Changed Room communication to use built-in `nexus_room` runtime tools instead of `nexusctl` Bash calls, and removed window controller/observer session-control roles from chat sessions.
- Refined conversation responsiveness with tighter narrow-column typography, shorter attachment hints, a collapsible left sidebar, and lazy-loaded Mermaid rendering.
- Updated the bridge SDK to v0.1.2 and defaulted pnpm registry configuration to npmjs for audit compatibility.

### Fixed
- Fixed built-in Provider settings so preset API format and Provider kind are derived internally instead of exposed as selectable controls.
- Fixed image-generation workspace artifacts so built-in `nexus_imagegen` MCP results produce image artifact cards, not only legacy CLI/Bash output.
- Fixed Agent deletion so removed Agents are hard-deleted with dependent database rows, preventing stale archived records from blocking name reuse.
- Fixed DM runtime startup so stale SDK resume IDs are cleared and retried once instead of leaving the client disconnected.
- Fixed group Thread opening while history, workspace, or about panels are active.
- Fixed shared WebSocket workspace subscriptions so sidebar task status and active chat workspace events do not cancel each other while switching between running tasks.
- Fixed desktop file actions, desktop update checks, WebView recovery, and Windows Claude Code runtime startup by bypassing npm `.cmd` shims and moving large system prompt/MCP payloads into local argument files.

## [0.1.12] - 2026-05-29

### Added
- Added DingTalk AI Tables, Tencent Docs, Yuque, DiDi, and AMap connectors, with remote MCP, token header, stdio token, or official MCP key configuration and runtime MCP mounting for Agents.
- Added DashScope and ModelScope provider presets with dedicated image-generation API formats; DashScope supports Anthropic Messages, Responses, and Chat Completions, while ModelScope supports Chat Completions.
- Added Skill community discovery and import from built-in sources, configurable JSON indexes, Git repositories, URLs, zip archives, and local files, with persisted source and import metadata.
- Added `nexusctl skill` support for external source search, Git import, one-shot external import/install, and imported Skill updates.

### Changed
- Refined Room collaboration around a minimal directed-message kernel: public Rooms advance through public `@` mentions, while private and small-group work use explicit `recipients`, `wake_policy`, and `reply_route`.
- Removed the standalone `nexus-migrate` binary and manual migration subcommands; database migration and Docker owner bootstrap now run through `nexus-server`, and frontend protocol generation uses `go generate ./internal/protocol`.
- Consolidated Skill import into a single dialog with source management, Git branch/path fields, local zip import, `SKILL.md` guidance, and Room Skill `scope: room` guidance.
- Changed `skills.sh` imports to clone the backing GitHub repository and import the selected Skill directory directly instead of depending on `pnpm dlx skills add`.
- Improved runtime MCP tool handling, connector credential flows, and service startup initialization, while reducing successful static asset and read-only request log noise.

### Fixed
- Fixed Room directed-message handbacks and public-feed wake-up routing so coordinators can return to the public flow through `next_reply_route`.
- Fixed DM and Room runtime fallback to the default chat model, escaped slashes in Provider model IDs, the GLM model list endpoint, and default model population for newly configured desktop-mode Providers.
- Fixed Provider configuration, Connector status, external Skill registry data, and summary counts so they are correctly scoped in multi-user deployments.
- Fixed Agent Skill dynamic discovery, `skills.sh`/Git/URL Skill import stability, external Skill search triggering, and temporary-directory-based naming.
- Fixed production copy failures and added clipboard fallback handling.

## [0.1.11] - 2026-05-27

### Added
- Added General settings roles for the default chat model, default image-generation model, and background task model, with background tasks such as title generation preferring the background task model.
- Added Custom Provider configuration, synchronization, and testing for Chat Completions, Responses, and Anthropic Messages, and exposed the OpenAI preset configuration.
- Added explicit `--provider` and `--model` overrides to `nexusctl imagegen`.

### Changed
- Refactored Provider default model selection and the lightweight LLM call path, while keeping the default chat model limited to Provider models supported by the current Agent runtime.
- Fixed built-in Provider Base URL and Models Path handling to use the built-in catalog, while the settings page shows Base URLs for all preset API formats and Custom Providers can still use custom endpoints.
- Aligned Agent prompt runtime context and workspace templates so built-in runtime constraints, default models, and tool usage guidance stay consistent.

### Fixed
- Fixed missing Skill selector title, excessive member list height, and oversized bottom spacing in the Room management dialog.
- Fixed Room member selection clicks.

## [0.1.10] - 2026-05-26

### Changed
- Refactored Provider configuration and default model selection: defaults now use explicit Provider + Model choices, Provider pages have complete localization, built-in Providers include Qwen Token Plan, MiniMax Token Plan, and Volcengine Coding Plan, and runtime no longer depends on the legacy `is_default` and `model` columns.
- Expanded long-running scheduled tasks with script execution, explicit member execution, run artifacts, stuck-run recovery, daily reports, per-task status, management events, history search, CLI operations, and runtime timeout watchdogs.
- Refined scheduled-task result delivery to support DM, Room, Agent inbox, Feishu, and other IM group destinations, with delivery ledgers, automatic retry, dead letters, manual redelivery, and historical traceability after task deletion.
- Allowed Feishu and external IM inbound messages to create, inspect, update, disable, delete, and redeliver scheduled tasks directly, backed by idempotent ledgers, signature validation, owner context, and managed Skills for observable and recoverable background handling.
- Added DOCX, XLSX, and PPTX workspace file previews, and improved Office preview layout, zooming, sidebar placeholders, PPTX master placeholders, and text style restoration.
- Added local user avatar settings for the desktop app, and added Windows update-check release notes.
- Added Codex built-in Skill reference analysis documentation to clarify reusable Nexus Skill ecosystem capabilities and implementation priorities.

### Fixed
- Fixed SQLite legacy migration startup failures, migration number conflicts, server single-file migration references, and test stability issues.
- Added an internal `[cron:...]` marker for scheduled-task trigger messages so the chat timeline hides automation-generated user trigger bubbles.
- Fixed scheduled task HTTP create/edit requests not accepting `execution_kind`, which caused page-created script tasks to be treated as Agent tasks by the backend.
- Fixed temporary Claude scheduling tools potentially accepting user reminders; reminders and long-running tasks now consistently require Nexus persistent scheduled tasks.
- Fixed Office file preview layout, table preview enlarged sidebar placeholders, XLSX zoom range, PPTX display, and PPTX text style restoration.
- Fixed the chat sidebar delete confirmation staying open after a failed delete request.

## [0.1.9] - 2026-05-23

### Added
- Added full Feishu Cloud Docs connector capabilities: user-managed OAuth Client configuration, callback URL copy, document read/create/append/block update, cloud space and knowledge base browsing, full-text search, Sheet reads, and Bitable record viewing.
- Added user-level memory management and Agent memory entry points, with search, filters, deletion, dirty-data cleanup, orphan session summaries, and checkpoint cleanup in contact details and the Memory page.
- Added deferred-loading metadata for MCP tools so connector and automation tools can return tool descriptions and input schemas on demand, reducing default context usage.
- Added Agent contact views so contact details and Room member panels can show DMs, requests, private notes, and small-scope record projections.

### Changed
- Refactored the web design system around shared Button, Dialog, Panel, SelectMenu, Avatar, ListRow, Badge, StateBlock, FormControl, Tabs, and related components, removing unused legacy components and excess Liquid Glass shells.
- Unified capability information architecture: connectors, Skills, message channels, pairing authorization, scheduled tasks, and memory pages now use lightweight directories, unified search and filters, detail pages, and consistent dialogs and empty states.
- Refined Feishu connector configuration by moving connector details from dialogs to secondary pages and reusing unified Dialog and Panel components for OAuth Client configuration and Device Flow authorization.
- Improved the DM/Room workspace with Safari-style conversation tabs, direct access from Room avatars to Agent contact information, and simplified new/manage Room dialogs with single-list selection.
- Improved Markdown streaming by delaying links for trailing URLs, tightening external-link protocol allowlists, and shortening displayed bare URLs.
- Unified page width, buttons, inputs, dropdowns, loading skeletons, and status feedback across settings, Agent configuration, scheduled tasks, memory, and capability pages.

### Fixed
- Fixed access logs potentially leaking query parameters such as `access_token`, `token`, and `api_key`, and added regression coverage.
- Fixed backend stability issues around WebSocket Origin validation, startup panics, file descriptor soft limits, session title refreshes, and Room public-feed projection coloring.
- Fixed OAuth callback windows not auto-closing after authorization success, connector lists not always refreshing, and overly broad nginx callback routing.
- Fixed help center close buttons, failed delete-session confirmation states, permission dropdown clipping, and file references being unclickable before the first workspace was opened.
- Fixed image generation landing in the wrong directory, oversized chat image previews, ordered-list marker overlap, automatic memory submission triggers, and low-value task memory extraction.

### Security
- Fixed the PostCSS security advisory GHSA-qx2v-qp2m-jg93, and tightened WebSocket Origin checks and access-log redaction.

## [0.1.8] - 2026-05-21

### Added
- Added a "Check for Updates" entry to the Windows desktop tray menu, allowing manual GitHub Release checks, downloads, and sha256-verified installation.

### Changed
- Made `make app-win-build` use the current timestamp as the Windows desktop app build number by default for local testing with uncommitted changes; `APP_WIN_BUILD_NUMBER` can still override it.
- Reduced GitHub `Publish Release` assets to macOS DMGs, Windows installers, and required sha256/metadata files, no longer uploading custom source archives, Linux/Windows binary packages, or Windows portable zips.
- Changed Windows desktop packaging scripts to prefer installers and locally produce only installer, sha256, and metadata artifacts by default.
- Refined Memory scheduling and API tests to improve regression coverage for dynamic recall, checkpoints, and HTTP APIs.
- Changed the Windows desktop app close button to hide the main window to the system tray; full exit now uses the tray icon context menu.
- Restyled the Windows desktop tray menu with a title, sections, and hover highlighting.

### Fixed
- Fixed onboarding completion state being lost on every Windows/macOS desktop launch when the sidecar local port changed.
- Fixed Nexus or DM entry clicks not opening the most recently active conversation.
- Fixed duplicate storage for the same attachment during send.
- Fixed Windows desktop auto-update checks writing the 24-hour throttle state before requests, causing failed checks to suppress later startup checks.
- Fixed Windows desktop Nexus motion being fully reduced to static text when system animation effects were disabled, and logged the reduced-motion state at startup for diagnosis.
- Fixed lingering Windows desktop shell and sidecar processes after closing the main window, which could block overwriting `.build/app/Nexus` during the next temporary build.
- Fixed Agent startup failures returning only generic WebSocket internal errors without Claude Code or Provider configuration guidance.
- Fixed Windows Agent runtime initialization when Claude Code installed through npm only exposes `claude.cmd` instead of `claude.exe`.
- Fixed Windows desktop log export failures caused by file-sharing locks on active sidecar log files.
- Fixed Windows WebView2 WebSocket handshakes being rejected with 401 when the `nexus_desktop_token` cookie was not written.

## [0.1.7] - 2026-05-20

### Added
- Added Nexus Memory v1 with local Markdown source of truth, automatic dynamic recall, candidate promotion, checkpoint deduplication, `nexusctl memory` commands, HTTP APIs, and a Web Memory panel.
- Added a notification loop after chat message completion: inactive windows can trigger browser system notifications, the left chat entry and conversation rows show unread completed-message counts, and counts clear automatically when entering the conversation.
- Added workspace file previews for Markdown, HTML, Mermaid, images, SVG, PDF, and plain text, with unified download entries in the preview area, chat file cards, and file context menu.
- Added GitHub OAuth Device Flow to the desktop app: release packages inject only the public Client ID, and the local sidecar polls and stores the token after the user enters the GitHub authorization code.
- Made desktop local mode skip account login by default and protect sidecar APIs through a native-shell-injected local session token.

### Changed
- Made `make logs`, `make logs-all`, and `make logs-nginx` show the latest 1000 lines by default for easier startup log inspection.
- Removed extra bridge SDK accessibility prechecks from the Makefile; installation, migration, protocol generation, and release package builds now rely directly on the Go module toolchain to validate dependencies.
- Removed frontend OAuth App self-configuration for connectors; the backend environment or desktop built-in configuration now decides whether connectors are available.
- Improved Markdown and preview streaming by separating stable blocks from streaming tails, aligning unclosed code fences to actual content, keeping the previous valid SVG for streaming Mermaid previews, skipping full highlighting during streaming code blocks, and reducing HTML preview reload jitter through head-readiness and throttled commits.
- Improved Markdown table rendering by correcting the formula/GFM table parse order and letting wide tables scroll inside their own container.
- Improved Markdown list rendering by fixing paragraph blocks that forced list-item content onto a new line after the marker.
- Improved Markdown text rendering with safe inline text tags, `<br>` line breaks, and better paragraph wrapping.
- Improved Mermaid SVG rendering with unified edge-label backgrounds, node radius, note colors, and diamond-node rounding.

### Fixed
- Fixed identifiers such as `Cron*(...)` in Markdown being misparsed as emphasis markers.
- Fixed workspace file editor/preview toolbar clicks on text regions triggering editor blur first and causing view jumps.
- Fixed workspace file status sometimes staying in "writing" after an Agent task ended.
- Fixed user message text not aligning by sender direction inside right-side bubbles.
- Fixed attachment preview paths becoming invalid after refresh when opening a user attachment accidentally focused the file tree on the internal `.nexus/attachments` directory.
- Fixed image attachments being sent to the runtime only as `@"path"` text, making first-turn image understanding unreliable, and aligned image content blocks to Claude Code `source.base64`.
- Fixed chat unread counts being stored only globally, missing from conversation rows, and not opening the corresponding unread conversation on click.
- Fixed the Windows installer incorrectly rejecting Windows 11 ARM64 running in x64 compatibility mode because of Inno Setup architecture constraints.
- Fixed desktop chat, sidebar subscription, and completion-notification WebSocket connections not carrying the desktop session token, causing local sidecar rejection.
- Removed GitHub OAuth Client Secret injection from desktop release packages to avoid exposing confidential client secrets in distributed artifacts.
- Fixed macOS Dock re-open resetting the current workspace route to the launcher.

## [0.1.6] - 2026-05-20

### Added
- Added the Windows desktop update download/install flow: a 24-hour-throttled GitHub Release metadata check can download `NexusSetup-*.exe` and sha256 files, verify them, and then prompt to launch the installer.
- Added Windows desktop Inno Setup installers to the release flow, producing `NexusSetup-<version>-<build>.exe`, sha256 files, Start Menu entries, optional desktop shortcuts, and `nexus://` protocol registration.
- Added the Nexus app icon to the Windows desktop app so packaged `Nexus.exe` displays an independent app icon.
- Added a native macOS "Check for Updates..." menu item that performs a 24-hour-throttled background GitHub Release check and prompts the user to open the download page when a new version is available.
- Added the first-stage Windows desktop WPF/WebView2 shell with Go sidecar launch, random local ports, runtime config injection, full launcher default entry, single-instance wake-up, `nexus://` routing, DPAPI credential keys, basic desktop bridge, diagnostic export, smoke scripts, zip/metadata packaging, and GitHub Release app asset upload.
- Added paste-image support to the conversation input and support for uploading images, PDFs, Office files, Markdown, HTML, and common text files as workspace attachments.

### Changed
- Unified desktop app runtime data under `~/.nexus`; macOS and Windows no longer use separate `Application Support/Nexus` or `%LOCALAPPDATA%\Nexus` locations.
- Changed chat attachments to pass structured metadata instead of appending file lists or excerpts to the message body. DM/Room pending queues and history replay now preserve attachment metadata, and Room attachments upload to conversation-level public directories.
- File tools now write structured workspace file artifacts after successful execution and expose a one-click open entry in chat.

### Fixed
- Fixed macOS desktop smoke tests treating `/login` as a startup failure when the app was not logged in.

## [0.1.5] - 2026-05-19

### Added
- Added Room owner configuration during Room creation and management, with an option for unmentioned public messages to be handled by the owner by default before replying or delegating to members.
- Added a macOS app build job to GitHub Release publishing, uploading dmg, sha256, and metadata assets to the same tag release.
- Added CI-friendly macOS desktop smoke fallback through launcher distributed notifications and configurable fallback reveal tolerance.
- Added a macOS app QA checklist and diagnostics for WebView external links/blocking, launcher close reasons, and WebContent termination.
- Added Makefile targets for macOS app development, build, run, smoke, and packaging.
- Added the Nexus concept app icon to the macOS desktop `.app` bundle.

### Changed
- Redesigned the sidebar chat workspace so contacts, capability entries, recent conversations, and the launcher console have clearer information architecture.
- Changed macOS app default launch and `nexus://launcher` to open the main window full launcher home, removed the separate compact launcher overlay, disabled the default `Option + Space` global wake shortcut, and removed launcher shortcut configuration from settings.

### Fixed
- Fixed Room slot state concurrent access risks and stabilized Room async cleanup tests.
- Fixed `nexus-server --help` triggering migrations too early.
- Fixed chat sidebar tab active state being lost after route changes.
- Fixed running macOS app instances not waking the launcher when opened again.
- Corrected macOS smoke validation for the default launcher route so startup and URL wake-up both land on `/`.

## [0.1.4] - 2026-05-19

### Added
- Added Nexus version display: release packages inject version, Git commit, and build time; `/system/version` returns current binary information; and Web settings link to GitHub Release downloads.
- Added Windows release package run instructions covering Claude Code, PowerShell, WinGet, and Git for Windows installation paths.

### Changed
- Agent workspace directories now use `agent_id`; renaming an Agent no longer moves the directory and only updates the database name and workspace `AGENTS.md` identity.
- Improved Windows compatibility for workspace initialization by adding a `nexusctl.cmd` entry and mirroring Claude Skill directories when directory symlinks are unavailable.
- Marked onboarding as read immediately when skipped to prevent the same tour from appearing repeatedly.

### Fixed
- Fixed release package launcher "Enter Workspace" clicks staying on the Launcher page.
- Fixed Agent renames failing on Windows when the workspace directory was in use.
- Fixed incomplete SQLite URL expansion for `~` and Windows path separators, and fixed database open failures when the SQLite parent directory did not exist.

## [0.1.3] - 2026-05-15

### Added
- Made release packages directly runnable: Linux and Windows runtime packages include the server, frontend assets, database migrations, and built-in Skills, and can serve Nexus through one local address after startup.
- Completed the image-generation capability with a dedicated image-generation Provider, built-in `imagegen` Skill, and in-conversation image result previews.
- Enhanced Room collaboration actions with private-domain messages, requests for specific members to reply, small-audience delivery, delayed wake-up, and room-level Skill rules.
- Completed the first internal validation stage for desktop: local sidecar, standalone window, desktop session credentials, startup diagnostics, and internal validation packages now have a closed loop.

### Fixed
- Made session running state rely on actually running tasks, reducing cases where conversations remained "active" after abnormal exit or failed interruption.
- Room deletion now cleans up members, sessions, messages, and execution records to avoid residual data affecting later use.
- Private-domain Room action sender identity is injected by runtime to prevent model-side spoofing or mistaken sender values.
- Private-domain actions no longer echo body text in tool results by default, reducing collaboration-process information leakage.

## [0.1.2] - 2026-05-12

### Added
- Added pending send queues to DM and Room inputs: when a conversation is running or already has queued messages, Enter enqueues new input, and queue items support manual guidance, deletion, and drag sorting.
- Added user-level default message behavior and default new-Agent permission mode to General settings. Default message behavior supports queue/interrupt only, and preferences are written to workspace JSON without adding database tables.
- Preserved the AskUserQuestion interaction channel in bypass permission mode while automatically allowing other tools.
- Replaced stale full session eviction with hot updates for conversation configuration: permission mode and model can switch in place, while changes that require reconnecting, such as cwd or MCP servers, are marked pending reconnect and applied automatically on the next request.
- Added Agent workspace Skill management, including installed Skill display, removal, and removal confirmation to prevent duplicate submissions.
- Improved scheduled-task flow with Agent selection and delivery count refresh.
- Added IM channel and pairing management with channel CRUD, pairing binding, and runtime plumbing, marked as unreleased preview.
- Unified backend API paths under `/nexus/v1`.
- Added Markdown preview/edit mode switching to the editor panel.
- Added `task_started` system message support with backend formatting and frontend presentation.

### Changed
- Removed inline "queue / guide / interrupt" choices from the input box; default message behavior is now controlled in General settings, and guidance remains only as a manual action on pending queue items.
- Reorganized General settings into Appearance, General, and Permissions sections with tighter copy and controls; preferences save immediately after selection, and permission settings are consolidated into four permission-mode dropdown choices.
- Changed DM and Room "guide" behavior into persistent queue state: guided items no longer disappear on click and are consumed only when the corresponding round's PostToolUse hook actually injects them.
- Replayed guidance message history from Claude transcript `hook_additional_context` instead of writing it into the overlay as a duplicate source of truth.
- Room public messages that mention a currently replying Agent no longer force-interrupt that Agent; busy targets receive extra context through SDK streaming input, while idle targets still start a new round normally.
- Room public context is now delivered as per-member cursor increments; fixed collaboration rules go into the SDK append system prompt, while per-round dynamic input keeps only public increments and a one-line natural-language trigger.
- DM conversations can accept additional input while replying, and new messages enqueue into the current streaming conversation instead of killing the active task by default.
- Simplified code block styling by removing red/yellow/green dots, reducing border radius, changing copy buttons to icon-only, and using horizontal scrolling instead of automatic line wrapping.
- Standardized frontend function and prop naming to snake_case across 126 files.
- Split frontend directories by feature domain, refining `types`, `hooks`, `lib`, `features`, and `workspace` into subdomains.

### Security
- Redacted SDK debug log content.

### Fixed
- Fixed guidance queues being consumed too early when the current round had no tool call, making messages neither injected nor visible.
- Fixed DM/Room rounds being treated as prematurely closed when the SDK returned no `result` but the assistant had already completed with `end_turn`.
- Fixed Room public follow-up context missing complete assistant replies without SDK `result`, and fixed manual guidance queue items being overwritten by public increments.
- Fixed guidance queues getting stuck under certain conditions.
- Fixed stuck DM streaming output.
- Added stronger diagnostics for Room round stream interruptions.
- Fixed database migrations not running automatically on service startup.
- Fixed a heartbeat state data race during concurrent access.

## [0.1.1] - 2026-04-25

### Added
- Refined the Room public collaboration mechanism with a `room-collaboration` system Skill, public `@` mention wake-up, follow-up `@` triggers after Agent public replies, and no-reply marker output filtering.
- Added personal avatar settings that reuse Agent avatar assets and synchronize avatars to profiles and login status.

### Changed
- Switched frontend and Docker deployment to pnpm: added `pnpm-lock.yaml`, removed `package-lock.json`, and updated the makefile, Web build image, runtime image, and in-container toolchain registry configuration.
- Changed Room public context to inject only public user messages and other Agents' final public results into Agents, no longer including tool calls, thinking, tool results, and other intermediate process data in other members' context.
- Restored Room input behavior to only restrict Agents that are currently replying; normal messages can still be sent while other Agents reply, and the Room Thread panel no longer closes automatically when result messages arrive.
- Allowed Agent renames that only change letter casing while still blocking truly duplicate names.

### Fixed
- Fixed Docker multi-stage builds where concurrent apt cache reuse could seize `/var/cache/apt/archives/lock` and fail installation.
- Fixed Docker builds where Corepack fetched pnpm metadata from npmmirror and received 404; builds now install a fixed pnpm version through npm.
- Fixed token usage data missing from settings when SDK JSON number types caused usage posting to be treated as empty.
- Fixed personal avatars not displaying in DM, the Room main message area, and Room Thread user messages, and ensured avatar changes trigger message item rerenders.
- Fixed Room rounds filtered by no-reply markers not writing token usage ledger entries.
- Fixed missing public results in Room public context injection and intermediate process data leaking into other Agents' inputs.
- Fixed new Room public messages interrupting the whole round by shared session; now only the explicitly mentioned target Agent is stopped.
- Fixed active Room interruption causing an early SDK stream close to be misclassified as a `round stream closed before terminal` error.

## [0.1.0] - 2026-04-24

### Added
- Landed the Go backend mainline with `nexus-server`, `nexus-migrate`, `nexusctl`, protocol generation, Goose migrations, and layered `gateway / protocol / runtime / chat / room / session / workspace / skills / connectors / automation` architecture.
- Added browser login and multi-user support with HttpOnly Cookie sessions, server-side session revocation, user-level main Agents, and data isolation for workspaces, rooms, sessions, Skills, and connectors.
- Upgraded DM/Room conversation flows with `transcript + overlay / transcript_ref` history as the source of truth, a shared round execution kernel, multi-observer single-controller execution, Room reconnect recovery, and permission-directed dispatch.
- Added the Capability area with a persistent Skill marketplace, structured scheduled task API/UI/MCP tools, heartbeat/cron automation runtime, GitHub Connector OAuth self-configuration, and `nexus_connectors` MCP tools.
- Expanded workspace and external entry points with workspace live subscriptions, file resource blocks, Discord/Telegram channel entries, and main UI capabilities for Agents, Contacts, Rooms, Settings, Scheduled Tasks, and Connectors.
- Upgraded deployment with Go multi-stage Docker images, an nginx gateway, production health checks, GitHub Release workflow, Agent toolchain bundled in runtime images, and Docker owner bootstrap.

### Changed
- Switched default development, build, migration, validation, and release flows to the Go backend; `make dev`, `make db-init`, `make check`, Docker, and release workflows now run around the current Go mainline.
- Refined gateway and business structure: HTTP handlers are split by domain, shared middleware moved into `gateway/shared`, and DM/Room/ingress/automation/WebSocket inbound routing is coordinated by `Dispatcher`.
- Consolidated session and history models: runtime no longer depends on the legacy `messages.jsonl` body path, session and room directories now use readable semantic paths, and history reads are bounded by Claude transcript and Nexus overlay.
- Made `nexusctl` Agent-friendly with global `--json`, `--pretty`, and `--verbose`, separated stdout/stderr responsibilities, unified success/error structures, and added `--password-stdin`.
- Reorganized the frontend around a unified same-origin API client, WebSocket binding semantics, conversation identity, runtime state machine, page-level controllers, and fuller onboarding/help entry points.
- Aligned automation tool parameters with the UI: `schedule`, `execution_mode`, `reply_mode`, agent scope, cron lookback, and lenient defaults now map to an editable and auditable task model.
- Updated documentation for the current architecture, including README, env examples, deployment notes, and reduced specs for session keys, permission runtime, main Agent, message processing, Skills, Rooms, and frontend design.

### Fixed
- Fixed runtime client invalidation, provider/model hot updates, `bypassPermissions` permission handling, tool parameter error display, file path display, SDK dependency prechecks, and Docker Skill root directory resolution.
- Fixed DM/Room inconsistencies around permission confirmation, stop generation, AskUserQuestion, multi-window observation, reconnect recovery, active-state detection, and input-box state.
- Fixed missing `nexus-manager` / `nexusctl` scope in multi-user deployments to avoid cross-user reads or operations on Agents, Rooms, sessions, workspaces, and Skills.
- Fixed local migrations, Alembic multi-head state, legacy auth-domain structure, Go migration detection, frontend dependency installation, and release workflows still referencing the old Python path.
- Fixed security and concurrency issues including Zip Slip path traversal, token timing side channels, sensitive configuration redaction, Resp global singleton mutation, bare `except`, and exception variable reference errors.

### Removed
- Removed the old Python runtime path, legacy sync/backfill, historical migration CLI, old workspace runtime layout migrations, cost-ledger backfills, and several old-field compatibility paths.
- Removed `messages.jsonl` as a runtime body source of truth, along with old session double-writes, old base64/short-hash directory layouts, and old result projection migrations.
- Removed the old frontend conversation store, home conversation controller, manual loading state, old StreamingCursor component, and stale Session/Workspace helper structures.

## [0.0.3] - 2026-03-18

### Fixed
- Fixed Markdown ordered lists rendering numbers and body text as separate lines in the message area, so content no longer breaks unexpectedly after `1.`.

### Changed
- Unified the main frontend visual style, moving the chat workspace, sidebar, status bar, input area, and empty states to one soft-neumorphic design language.
- Unified internal message block styling so `thinking`, tool execution blocks, Q&A blocks, code blocks, and message statistics share concentric radii and consistent panel hierarchy.
- Unified configuration and confirmation dialog styles so `AgentOptions`, permission confirmations, and confirm/input dialogs match the main UI.
- Refined radius, borders, and shadow rhythm for remaining task overlays, Markdown tables, and related components to reduce visual fragmentation.
- Added SQLite ORM models and an initial Alembic migration for `Agent / Profile / Runtime / Room / Conversation / Session`, establishing the new in-app collaboration data skeleton.

## [0.0.2] - 2026-03-17

### Fixed
- Fixed Agent deletion only archiving records without reclaiming workspace directories and active sessions, leaving old workspaces behind.
- Fixed `thinking` blocks disappearing after later assistant snapshots arrived; thinking blocks now remain stable in the same message round.
- Fixed `tool_result` being split into standalone assistant bubbles; tool results now render back inside the corresponding assistant segment.

### Changed
- Rewrote the backend message processor into a thinner `ChatMessageProcessor + AssistantSegment + SdkMessageMapper` structure aligned to the SDK's actual message rhythm.
- Tightened frontend streaming boundaries so only `thinking / text` participate in `StreamMessage` incremental rendering, while tool calls and tool results use full message snapshots.

## [0.0.1] - 2026-03-14

### Fixed
- Fixed delayed frontend display caused by a second typewriter animation over `thinking` and text streaming content, restoring immediate rendering from backend chunks.
- Fixed unstable ordering when assistant segments closed, tool results were inserted, and the same `message_id` was updated in the message streaming path.
- Fixed frontend errors in `TodoWrite` extraction, session deletion, and workspace sidebar rendering for empty blocks or empty `session_key` cases.

### Changed
- Refactored message protocol boundaries by adding `StreamMessage` and unifying backend streaming messages, final messages, and frontend consumption models.
- Adjusted WebSocket/IM sending layers to explicitly separate `message`, `stream`, and `event` transports.
- Passed `include_partial_messages` to the SDK by default and removed invalid frontend streaming/round configuration options.
