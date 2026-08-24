# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added bundled Word, PDF, PowerPoint, and Excel Skills with task-specific reading, creation, editing, conversion, and delivery workflows.

### Fixed

- Switched Windows WebView2 minimize and tray handling to its supported visibility lifecycle, preserving desktop and keyboard input after updates and window restores.

## [0.1.37] - 2026-08-24

### Added

- Added Nexus Browser with guided Chrome and Edge setup, a visible action cursor, Session-scoped tabs, inherited popup and OAuth tabs, batched actions, page inspection, file transfer, screenshots, PDF export, and an explicit Ask Nexus handoff.
- Added reusable WorkGraph drafts and sketches with history, immutable versions, conversational editing, message cards, Slash and Composer discovery, and cross-Session reuse.
- Added opt-in Echo follow-ups and generated greetings for first Agent and Room conversations.

### Changed

- Unified dialogs, Capability pages, settings, forms, pickers, and desktop typography into quieter responsive surfaces, with refined macOS window chrome and DMG layout.
- Reworked DM and Room process progress into compact expandable summaries, improved streamed Markdown pacing, and simplified Room Agent controls.
- Rebuilt managed CLI Skills around on-demand domain guidance and updated the Agent SDK bridge to v0.1.30.

### Fixed

- Stabilized indexed history, round navigation, scrolling, Room streaming, queued interjections, reconnect retries, and recoverable conversation errors.
- Hardened WorkGraph editing and saves across isolated hidden sessions, restarts, immutable version selection, and durable Plan proposal binding.
- Fixed Browser tab and reference lifecycle, command validation, bounded snapshots, pointer input, and desktop-only availability.
- Fixed late-created Windows WebView input windows, macOS resume probes and DMG assembly, default Provider selection, and compact responsive label clipping.
- Propagated Provider model output limits to nxs, including the documented 384K output limits for DeepSeek V4 Pro and Flash.

## [0.1.36] - 2026-08-19

### Added

- Added indexed DM and Room history windows, exact large-result detail reads, and durable single-Agent conversation branching from completed replies.
- Added owner-scoped custom STDIO, HTTP, and SSE Connectors with encrypted secrets plus Agent defaults and per-Session selection.
- Added desktop multi-folder workspaces and native file actions for opening, copying paths, and attaching workspace files.
- Added Agent contacts with Room-backed private conversations, contact management, history, and existing realtime chat behavior.
- Added streamed inline Generative UI through `/visualize`, together with safer widget isolation and clearer runtime failures.
- Added secure Automation input files, standard Cron expressions, recipient-aware permissions, and delivery across Nexus and supported IM sessions.
- Added a Word-reading Skill, Community Skill recommendations, and user-facing architecture documentation with standalone diagrams.

### Changed

- Replaced Goal and Execution MCP schemas with managed Skills and round-scoped CLI commands backed by exact authority, receipts, and lifecycle state.
- Replaced the Automation MCP mutation surface with a managed Skill and inspect/plan/apply CLI flow, while simplifying task context, routing, and result presentation.
- Made marketplace Connectors opt-in per Agent or Session and forked runtime sessions when their model-visible tool surface changes.
- Reduced always-on Agent and Room prompt content and moved configuration, management, visualization, Goal, Execution, and Automation detail into managed Skills and host CLIs.
- Improved conversation activity, history navigation, long-message presentation, Runtime Graph detail grouping, WorkGraph review loops, and responsive Goal controls.
- Replaced frequent frontend, desktop, scheduler, watcher, orchestration, and recovery polling with event-driven invalidation, durable wakeups, and bounded audits.
- Updated the bundled nxs runtime and Bridge integration for compact prompts, exact session forks, scoped result references, explicit permission boundaries, and managed runtime persistence.

### Fixed

- Made Goal and WorkGraph authority, continuation, review, completion, token accounting, and restart recovery durable across DM and Room collaboration.
- Stabilized scheduled-task execution, permission approval, result projection, recipient routing, Agent changes, and deleted or unpaired IM sessions.
- Protected conversation forks, pending branches, indexed history generations, round ownership, queued input, and retry flows across navigation and cancellation.
- Stabilized Room streaming, handoffs, working indicators, private-message filtering, Agent contacts, and shared WebSocket recovery without duplicate wakes or replies.
- Preserved isolated-runtime ACLs and argument-file ownership, routed process signals through the trusted launcher, and bundled the nxs ripgrep sidecar for Linux and desktop builds.
- Hardened desktop resume and sidecar cleanup, database migration numbering and repair, dotenv parsing, private Skill credentials, and workspace/runtime path handling.

## [0.1.35] - 2026-08-10

### Added

- Added conversational configuration management with role-scoped guidance, native approvals and secret entry, auditable versioned changes, immediate revocation, and explicit hot reload.
- Added owner-scoped private Skill sources with authenticated search, checksum-verified imports, online updates, and conversational management.
- Added one durable Execution WorkGraph across Goals, Plans, Room assignments, Subagents, tools, gates, retries, reviews, and artifacts, with atomic proposal/commit and adaptive promotion.
- Added task-scoped scheduled automation permissions with persistent approvals, connector reauthorization, safe retry, and durable recovery.
- Added Room member pause/resume and stop-all controls, opt-in per-round Agent emotion context, and adaptive buffered Markdown streaming.

### Changed

- Reworked WorkGraph into a responsive real-time canvas with an activity dock, bounded run history, ownership hierarchy, pan/zoom/search/inspection controls, and WebSocket-driven refresh.
- Simplified task and Subagent navigation, configuration flows, Provider setup, native desktop update launch, and responsive conversation controls.
- Localized conversation, Room, Agent, Skill, automation, workspace, tool, time, and accessibility surfaces across Chinese and English interfaces.

### Fixed

- Hardened Goal, Execution, Plan, and WorkGraph identity, migration, admission, concurrency, retry, replan, and recovery behavior under duplicate or out-of-order runtime events.
- Preserved exact Subagent and Tool histories, ownership, artifacts, terminal states, review returns, and bounded graph completeness across recovery and compaction.
- Preserved Room and DM ordering, unread positions, pending questions and permissions across reconnects, exact stop acknowledgements, and concise non-duplicated runtime errors.
- Kept QR-to-verification-code Channel authorization transitions atomic across background polling and foreground status checks, preventing stale QR cards from replacing secure code input.
- Completed Session, Room, Agent, scheduled-task, imported-Skill, transcript, summary, and Subagent artifact cleanup without leaving owner-scoped data behind.
- Tightened configuration authorization, runtime isolation, scheduled-task approval fencing, and legacy database migration recovery.

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
