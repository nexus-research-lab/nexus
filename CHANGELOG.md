# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Removed the automatic Goal resume confirmation; explicit objective replacements now reactivate paused, blocked, or usage-limited Goals directly while budget-limited Goals still require a budget change and the inline Resume action remains available for unchanged Goals.
- Passed each model card's context window to nxs so automatic and reactive compaction use the configured Provider limit instead of relying on model-name inference.
- Allowed custom Provider display names to use Chinese and other Unicode characters without losing their internal key, and surfaced actionable subscription Token quota errors in Web conversations and queued delivery.
- Prevented Room's internal no-reply marker and synthetic result-only reply from leaking into public messages, streaming previews, or result summaries while preserving terminal usage accounting.
- Fixed DM and Room queue/guide delivery so user corrections stay hidden and deletable until consumed, hook-applied ACKs commit only their exact batch, DM queues continue as durable next turns, and each Room Agent immediately consumes its own queue without waiting for peers.
- Fixed DM and Room Goal retargeting so corrected objectives reach active runtimes, revisions advance only after guidance consumption, stale continuations cannot launch or mutate corrected Goals, and continuation reserve/claim/release accounting remains exact across duplicate dispatches.
- Fixed Markdown image destinations being wrapped as inline code, restoring relative workspace image previews in historical conversations.

### Added

- Added a dedicated visual-model preference and nxs runtime projection, allowing text-only chat models to call an independently configured vision provider on demand while native vision models keep the direct image path.
- Added Developer ID signing, notarization, GitHub Actions release and CI package validation wiring, and signed metadata support to macOS desktop packaging so trusted releases can pass Gatekeeper and automatic-update checks.
- Added durable Room delayed wakes, causal message metadata, separated visibility and wake targets, bounded per-Agent wake queues, and compatible short-window wake batching.
- Added durable scheduler leadership leases, deterministic recurring jitter, configurable misfire handling, per-user enabled-task limits, and explicit task expiration with audit events.
- Added per-Agent non-secret nxs settings projection, including enabled Summary/AutoMemory/AutoDream defaults, and a host memory-maintenance coordinator that resolves owner provider credentials and background models before waking AutoDream.
- Preserved native `system/memory_saved` events and AutoDream written paths through the bridge and Nexus message projection.
- Added a file-backed Memory view to Agent contact details, with `MEMORY.md` index navigation, typed topic memories, daily logs, index coverage, freshness warnings, search, and direct Markdown editing.
- Added a capability-driven subagent inspector for DM and Room conversations, with active/completed task visibility, multi-round transcripts, stopping, follow-up messaging, same-task resume, and explicit `nxs`/Claude Code support boundaries.

### Changed

- Refined IM pairing management with pending-first status views, accurate local filtering, action-oriented empty states, and collapsible technical details.
- Filled missing context windows for commonly used model families from a maintained model-card catalog while preserving Provider-reported values as authoritative.
- Replaced independent Room history limits with a model-window-aware context budget, product-owned cold-start anchors, prioritized public/private deltas, and checkpoints that advance only through actually consumed content.
- Kept Room runtimes warm until the shared idle reaper closes them, reduced Room communication tool acknowledgements and system prompt size, and made Room Skills inject explicit runtime instructions instead of full README bodies.
- Reworked scheduled tasks into a full-height status-driven board with independently scrolling columns, concise task cards, and guided presets for the empty state.
- Added a dedicated one-time workspace migration to remove legacy Nexus-managed `memory-manager` Skill deployments after memory moved into `nxs`, while preserving user-authored Skills with the same name.
- Removed the workspace-panel preload dependency from generated image rendering, resolving relative workspace image paths directly through the authenticated preview endpoint.
- Displayed the effective Agent runtime in the Composer footer, preferring the conversation runtime over the current default.
- Surfaced live context-compaction status from Claude Code and nxs in the conversation Composer while retaining the completed compact boundary in history.
- Enabled Tool Search across Providers through native references or SDK schema promotion, kept interaction and web tools inline, and deferred all Nexus MCP schemas behind discovery.
- Consolidated the scheduled-task MCP surface from fourteen backend-shaped tools into eight intent-level tools, removed the duplicate scheduled-task Skill through the shared one-time workspace migration ledger, and made models discover the tools directly through Tool Search.
- Split the shared Select menu into pure presentation, controller, and view ownership; removed stale memoization and type forwarding, fixed unknown-value keyboard traversal, and made nested overlays consume Escape before parent dialogs.
- Made the selected workspace path the single preview state, removed unreachable standalone preview and resize modes, and moved the shared panel resize handle under layout ownership.
- Replaced the shared workspace surface header boolean matrix with explicit page, overlay, and accessibility-only modes, removing unsupported title-slot combinations.
- Split Markdown workspace artifact parsing from Agent store hooks, indexed file lookups once per file set, and preserved surrounding message text when projecting absolute file paths.
- Replaced branch-heavy Markdown text AST mutation with ordered stateless break and inline-HTML conversion rules, removing shared regular-expression state.
- Consolidated DrawingML transform rectangles, group bounds, geometry presets, and fill-color sources under presentation-owned readers with explicit validity rules.
- Split PPTX slide inheritance, shape resolution, and preview filtering into explicit phases, restoring placeholder stroke-width fallback when a shape has no local line definition.
- Moved spreadsheet CSS projection out of the virtual grid model and into the cell-style domain, removing an unused border-type forwarding surface.
- Replaced PPTX shape-tree branching with node handlers, privatized parser-only state, and split spreadsheet virtual-grid projection into explicit viewport, cell, merge, and layout phases.
- Split Agent Options skill loading states and list sections into narrow views, and separated scoped skill command creation, mutation, stale-result rejection, and cleanup phases.
- Replaced Agent Options mode and optional-initial-value matrices with explicit create/edit sources, reused one Agent projection across Contacts and Room, and split Skill refresh lifecycle phases into pure state transitions.
- Replaced Agent identity name-validation JSX candidates with ordered semantic feedback rules and a single tone renderer.
- Consolidated Provider Settings presentation rules, removed duplicate custom-provider state, added icon fallbacks, and split configuration, header, model-row, capability, and toggle rendering responsibilities.
- Centralized Goal form and controller projections, unified confirmation rendering, and split command transactions into explicit lifecycle phases.
- Split Memory summary and document body states into narrow views, and made save scope snapshots and draft-merge rules explicit.
- Unified Memory Agent and document scope guards, projected catalog sections and document actions from pure models, and moved `MEMORY.md` index parsing into its owned subdomain.
- Centralized Skill catalog source, install-state, and update-check projections, removed a forwarding badge wrapper, and turned Markdown deduplication into an ordered model pipeline.
- Centralized Channel card and QR-login state projections, replacing duplicated JSX conditionals with discriminated actions and session views.
- Separated General Settings resource controllers from Sections, split default-model catalog projection from preference normalization, removed obsolete provider-response fallbacks, and made desktop version failures leave loading state.
- Centralized Markdown code, link, and streaming URL-tail semantics into ordered pure models, leaving renderers to route discriminated presentation states.
- Split conversation navigation into explicit round snapshots, visible DOM candidates, and an owned frame-landing runtime; centralized timeline visibility and scroll-content identity rules.
- Unified DM and Group Chat panel environment, frame, and activity-snapshot projections, tightening their single-consumer contracts and removing empty command and layout defaults.
- Separated Room round-card views from Thread state and live data, made Agent directories concrete end to end, and replaced branch-heavy round and error status classification with ordered rules.
- Moved Subagent list grouping and support state plus Todo round display decisions into pure projections, replacing repeated filtering, capability fallback fields, and branch-heavy task selection.
- Unified required-field and enum decoding across conversation, Session, ACK, and Workspace payloads, and consolidated Resync cursor advancement plus reload-before-resubscribe lifecycle.
- Replaced conversation action validation with ordered guards, separated stream metadata from content updates, and unified runtime snapshot and terminal-message reconciliation phases.
- Moved Agent conversation option normalization and public result projection out of the assembly Hook, using a default identity sentinel and removing its unused type-forwarding surface.
- Replaced the branch-heavy Assistant image identity fallback chain with an explicit ordered resolver table while preserving snapshot merge priority.
- Unified virtual-feed height inputs into one role and content-block projection, removing duplicate message scans and obsolete string-content compatibility branches.
- Centralized Room Session key strategy and snapshot field fallback in pure models, leaving the snapshot Hook responsible only for applying projected effects.
- Moved shared message history and round-index contracts out of Room types, unified Room and Session pagination serialization and response normalization, and separated Session HTTP calls from response transforms.
- Moved Room dialog defaults, remount identity, and mode labels into a pure model so the dialog view consumes one concrete content contract.
- Replaced conditional rail and panel primary-tab rendering with one button structure driven by an explicit presentation table.
- Replaced branch-heavy chat notification target and route resolution with ordered identity rules while preserving Room fallback and Session isolation.
- Split workspace conversation-tab reconciliation into explicit retain, append, and fallback phases, and moved single-tab state styling into a pure presentation model.
- Split scheduled-task source and submit transactions into explicit phases, made daily Cron parsing reject partially invalid expressions, and centralized run actions plus list-item state in ordered presentation models.
- Centralized Icon Picker, List Row, Glass Switch, and Workspace Tree Row presentation and interaction rules in shared models and Hooks, removing duplicated disabled guards and render-level state matrices.
- Replaced Contacts page optional-field assembly and Agent private-thread render branches with explicit page-state and density-aware thread-list projections.
- Moved connector Device Flow polling into an owned cancellable state machine, and serialized subscription loading plus mutations while centralizing feedback and view projections.
- Split login redirect policy, Auth orchestration, and form rendering into owned layers, rejecting external and login-loop redirects while consolidating desktop render snapshot sentinels.
- Split presentation text rendering into shape, paragraph, and run stages, and centralized Launcher recent-entry type styling in a dedicated presentation model.
- Split PPTX package orchestration from Shape Tree parsing, caching shared layout and master parts so repeated slides reuse inherited elements and image resources.
- Removed render-phase Thinking expansion mutations and separated image-source normalization, user-attachment capability projection, and embedded tool-error parsing into short owned phases.
- Moved message ordering, system-event, permission, result, and final-response projection into an owned controller subdomain, reusing the single ordered-entry projection and replacing content-mode branches with exhaustive strategies.
- Split message content rendering into entry selection, structured orchestration, exhaustive block routing, and tool interaction phases, while consolidating the shared AskUserQuestion tool identifier.
- Moved message activity state out of UI primitives, consolidated round and content activity rules under an owned pure domain, and separated process summaries and question timeout detection from the controller aggregate.
- Split the mixed message UI primitive aggregate into directly owned avatar, action, activity-status, and message-item shell responsibilities, consolidating activity presentation under one exhaustive state table.
- Replaced branch-heavy message result and usage formatting with ordered display and numeric-range rules plus independent duration, token, cost, and cache projections.
- Consolidated modal stacking, body scroll locking, focus discovery, focus cycling, and Escape handling under shared Dialog ownership; removed duplicate business-level keyboard listeners and migrated Agent Options, Mermaid preview, and scheduled-task run history off their private modal shells.
- Moved Confirm and Prompt dialogs into an owned decision subdomain, sharing one modal frame and action bar while replacing Prompt focus/reset effects with mount-scoped state and ordered keyboard rules.
- Moved Guide Center modal ownership into Onboarding, replaced interactive Article cards with semantic actions, and made Shopify domain input a controlled Auth transaction instead of a temporary React Root.
- Separated scheduled-task run diagnostics from history status and action rules, using ordered definitions for detail rows, output sections, and copied diagnostic fields instead of repeated conditional assembly.
- Moved ExcelJS cell-value classification and text formatting into an owned Spreadsheet value model, replacing the branch-heavy `unknown` object probe chain with ordered rules over the real closed `CellValue` union.
- Split Room history items into pure activity/action projection and mode-driven reading/editing views, replacing repeated active, capability, metadata, and title-state conditions in the item entry component.
- Split Skill import into a thin dialog shell, Git draft controller, mode-driven source views, static authoring guide, and Footer commands, replacing the branch-heavy component that mixed focus, submission, upload, documentation, and presentation state.
- Removed the single-consumer shared MultiSelect abstraction and moved Room skill loading, filtering, multi-selection, search, and asynchronous menu states into an owned `members/skills` domain while retaining only reusable overlay and listbox infrastructure in shared UI.
- Moved the Agent private-domain timeline into an owned subdomain with exclusive body states, typed direction styling, and pure event/header projection; tightened private direction and scope contracts to the backend's closed enums and removed the ambiguous shared presentation model.
- Split the text file editor into pure mode and toolbar projection, Header, and Body ownership, separating external-write status, renderer selection, element sizing, and textarea focus from entry orchestration.
- Reworked scheduled-task edit initialization into execution, target, and reply projections, normalizing hidden fields that are invalid for Script or Room tasks instead of carrying stale combinations into the form draft.
- Moved capability navigation into an owned sidebar domain with table-driven item projection and consolidated summary revalidation, removing an unused shared row API and the duplicate scheduled-task mutation event.
- Split User message presentation into density projection, header actions, identity, content, and editing composition, while preserving concrete role types through assistant-content merging, separating deduplication phases, and removing unconsumed text projections.
- Replaced the unused multi-banner Stack contract with a single feedback viewport and table-driven tone presentation, resetting auto-dismiss timing when feedback content changes.
- Split message file and image artifacts into scoped source and presentation models, consolidating path parsing and browser-download or desktop-reveal behavior into one Artifact-owned action.
- Split Composer input, submit, Footer, pending-queue, Loop-picker, and attachment-paste responsibilities into scoped models, controllers, and views; removed optional handlers unsupported by its real DM and Room consumers.
- Split Composer state derivation into input, runtime, mode, and action projections; separated message submission phases and ordered IME, Safari, and Mention keyboard guards while keeping the panel contract flat.
- Moved permission event decoding into its own transport subdomain, replacing repeated optional-field branches with typed enum, scope, suggestion, and default-interaction projections before handlers mutate Session state.
- Split AskUserQuestion cards and control flow into scoped presentation, draft restoration, interaction state, and submission transaction modules, removing branch-heavy JSX and preserving stale-scope submission isolation.
- Split Assistant message rendering into scoped layout, header, content, process, and permission views; unmatched permission requests now render exactly once instead of borrowing or duplicating process content placement.
- Split tool execution blocks into scoped interaction control, a single status model, and header projection/actions, removing mirrored running/waiting flags and branch-heavy JSX state interpretation.
- Replaced the optional-mode Agent Options editor with explicit inline and dialog shells, removed the unconsumed inline navigation path, and separated tab content, footer actions, and scoped save transactions from controller assembly.
- Split Connector detail state, header actions, status content, and feature selection into owned modules, replacing eager JSX action matrices and repeated OAuth/configuration conditions with explicit state rules.
- Split the scheduled-task basics form into target/session presentation, reusable choice fields, and small advanced-setting sections instead of maintaining one branch-heavy component with repeated selectors and inline copy rules.
- Split Skill detail routing, scoped resource loading, presentation projection, and pure rendering, while reusing marketplace update/delete commands with explicit success results instead of duplicating API and feedback transactions inside the view.
- Removed the unused stacked Workspace Header mode and trailing-tab contract, standardized its shared single-row height, and separated identity, navigation, and toolbar action ownership instead of maintaining an optional-prop layout matrix.
- Moved shared tool activity labels and input summaries out of the concrete Tool Block model, removed unused view-model state, and returned permission labels, status styles, and detail scrolling to their owning projections and views.
- Moved subscription operations under the Settings domain to remove the top-level Settings/Operations dependency cycle, consolidated tab metadata and content selection into one definition table, and promoted shared Agent Options rules out of the editor Feature so Config and sibling Features depend only on the base library.
- Replaced the mixed runtime options entry with separate endpoint, conversation-policy, scoped-snapshot, and application-resource modules, and moved authentication transactions out of the shared Context layer.
- Removed the remaining TypeScript import cycles by moving Agent conversation identity projection out of protocol types and separating Onboarding contracts from the Context and Provider implementation.
- Organized frontend API clients by Agent, Account, Capability, Conversation, Settings, and Core ownership, renamed the ambiguous conversation `agent-api` to `session-api`, merged duplicate runtime settings clients, split Room queries, commands, projections, directory events, and navigation recovery to remove the API-to-Store dependency cycle, removed unused app-update declarations and the dead bulk Skill update client, and made file-private declarations stop leaking as public exports.
- Split the core HTTP transport into request preparation, response projection, error types, authentication events, and fetch orchestration, removed forwarding imports, and guaranteed abort-listener cleanup when response parsing fails.
- Organized shared UI primitives by button, form, display, list, and navigation ownership instead of keeping unrelated controls in one flat root.
- Split oversized Chinese and English message catalogs into typed domain segments with per-segment key parity checks.
- Removed matching and presentation logic from conversation type declarations, consolidating permission matching and returning question, system-event, and Surface Tab rules to their owning domains.
- Consolidated identical cross-feature error-message wrappers into one stateless library projection while keeping domain feedback composition local.
- Split scheduled-task types into task and run contracts, removing unused status, event, health, and daily-report declarations instead of preserving speculative API mirrors.
- Split conversation messages into attachment, content, entity, and event contracts, removed the root type barrel and unused message declarations, reused generated WebSocket envelopes, and added owner-specific decoding for previously unchecked event data.
- Removed the frontend utility grab bag, routing class-name composition directly to its UI primitive while separating relative-time, token-count, and avatar rules into owned modules with data-driven projections.
- Separated Liquid Glass capability, interaction, animation cleanup, and SVG filter ownership, removed render-phase state writes, and deleted message and shared-UI forwarding barrels in favor of direct responsibility imports.
- Removed the remaining frontend protocol `any` escapes and dead SDK aliases, sharing unknown tool inputs across messages and permissions while validating AskUserQuestion payloads inside the owning feature.
- Consolidated Composer attachment classification, accept filters, rejection localization, and Agent/Room upload preparation into one data-driven pipeline that validates whole batches before side effects.
- Split channel QR login state, polling, status projection, and verification UI from channel configuration transactions while preserving one shared command lock; separated the connection dialog shell, fields, footer, and presentation projections behind consumer-defined contracts; and removed the single-export Agent Hook forwarding entry.
- Consolidated Select and MultiSelect overlay state, trigger keyboard behavior, listbox framing, and data-driven size/surface projections while preserving their distinct selection semantics.
- Replaced inferred Conversation Session and Room label Hook types with consumer-defined navigator, viewport, feed, and translation contracts, and reduced visible-window revisions to their explicit count inputs.
- Split the scheduled-task directory and run history into scoped resources, mutation commands, overview, dialog shell, and narrow list/detail/action views, reusing the shared anchored action menu for task actions.
- Consolidated action, select, and multi-select menus under shared menu ownership, and unified scheduled-task time columns and anchored overlay behavior across pickers.
- Split the Home ASCII Hero into presentation, React lifecycle, Canvas scene, and particle-model modules.
- Made Nexus the sole owner of scheduled-task state and timing, exposed automation to runtimes through the injected MCP server, and simplified task creation, list actions, and run diagnostics around progressive disclosure.
- Renamed the product scheduling domain and storage from Cron jobs to scheduled tasks; Cron now denotes only the `schedule.kind=cron` expression mode.
- Moved Room page state into page-owned scoped data, projection, command, snapshot, and existing-Agent option controllers, while separating URL navigation, one-shot drafts, server events, and Tours from the view entry.
- Split Contacts into page-owned Agent resource/editor commands and URL navigation, replacing three conflicting dialog states with one tagged state and reusing the shared Agent Options mutation boundary across Contacts and Room.
- Split root bootstrap into startup orchestration, React root rendering, shared failure presentation, chunk/auth recovery, reload guards, and blank-render watchdog modules.
- Split Room conversation history into pure entry projection, a single-state title editor, item and empty-state views, while moving its private delete policy out of the shared conversation protocol layer.
- Split Workspace Catalog shared UI into card frames, content primitives, actions, and icon media instead of exposing unrelated controls from one aggregate module.
- Collapsed Room member management into one page-scoped command and split the Group header's member stack and guide menu into narrow presentation modules.
- Unified DM/Group header tabs and guide actions, and split the mobile Room surface into header, conversation sheet, thread, subagent, and shared chat modules.
- Split Group Chat Panel orchestration into session, composer, Goal, projection, and view modules while removing its constant control flag and display-string duplication.
- Moved the Room Thread live bridge out of Chat and the global store into a private Thread domain with separate source, projection, producer, and consumer modules.
- Split Room round cards into one-pass projection and narrow reply/action views, removing dead feed props, repeated permission scans, and duplicated Thread controls.
- Split the DM Chat Panel into Goal, session, Composer, projection, and view modules while sharing identical navigator, viewport, and scroll-control projection with Room.
- Moved the Workspace file tree to its shared owner, split hierarchy and row projection, replaced extension branches with a visual lookup table, and rendered context-menu commands from action data.
- Split Room Workspace interaction state, path navigation, and entry transactions from its orchestrator, replacing the flat controller surface and leaked state setters with consumer-owned controls.
- Split Room Workspace orchestration into Agent scope, file resource, path model, command, layout, file-browser, and dialog modules.
- Split conversation Todo projection into round indexing, runtime-task merging, status inference, and a stable shared hook used by both DM and Room panels.
- Consolidated conversation scrolling under the timeline domain, separating smooth animation, user interactions, history-prepend anchoring, local expansion anchoring, and round navigation protocols.
- Split Agent message processing into Assistant content identity, message collection, and stream reducer models instead of sharing one cross-layer helper module.
- Split Agent conversation runtime into pure models, volatile snapshot storage, and narrow React state controllers, removing the state type cycle and unused runtime surface.
- Split WebSocket transport into connection policy, heartbeat, client, shared-channel registry, and React lifecycle modules, with shared identity derived from the complete effective configuration.
- Split Workspace conversation tabs into pure ordering/layout models, a browser-style tab transaction controller, and a presentation-only tab item.
- Split Agent memory browsing and document editing into scoped resource controllers, with a presentation-only catalog and explicit live SDK content precedence.
- Consolidated Agent Options identity, model, and permission fields into one scoped draft, with dedicated Provider, name-validation, and save-feedback controllers.
- Split Agent identity presentation into profile, vibe-tag, model-selector, and layout modules, removed duplicate dialog/inline form trees, and reset pending tag input across editor scopes.
- Split Agent skill configuration into scoped resource, command, search projection, and presentation modules, cancelling stale list requests and serializing writes per Agent.
- Split AskUserQuestion into an atomic answer model, tool-scoped interaction controller, and status-driven presentation, preventing duplicate submissions and stale result hydration.
- Replaced the message-item support grab bag with owned content, time, projection, activity, and timeline modules, and converted message ordering into an explicit staged pipeline.
- Removed the flat message-item state contract, grouping User and Assistant state by visual responsibility while letting each view define its own narrow input contract.
- Split message-item display projection and process-expansion lifecycle from controller assembly, replacing the inferred projection-hook dependency with a consumer-defined structural contract.
- Split conversation navigation into timeline data projection and ruler presentation models, unifying loaded and indexed rounds behind one navigation-item projection.
- Decomposed round jumping into scoped target, serialized window-loading, and DOM landing controllers, preventing stale requests from affecting newer navigation.
- Split visible timeline window loading into candidate selection, retry runtime, and scheduling controllers while keeping its opportunistic strategy distinct from explicit round jumps.
- Split Provider configuration actions into field linkage, persistence, enablement, and deletion controllers, exposing only a narrow persistence command to model and connectivity-test actions.
- Split Provider model actions into scoped state, sync, add, update, and test commands, centralizing persisted-request orchestration and default-model protection.
- Split Provider workspace state transitions from resource loading and added request-generation guards so stale refreshes cannot overwrite the active visibility scope or selection.
- Moved Provider catalog, format, title, and capability presentation into a pure projection and returned custom-provider name/key linkage to the configuration field controller.
- Moved Agent message collection, chat-ack failure, and event-dispatch lifecycles out of the public conversation hook, while keeping public actions stable against streaming state updates.
- Split Agent session orchestration into identity migration, background/volatile snapshots, lifecycle context, and command composition, removing a redundant input-queue setter wrapper.
- Split Agent WebSocket handlers by message, permission, resync, session, and scope event families, centralizing current-session guards and rejecting duplicate event ownership during router registration.
- Split subagent threads into scoped transcript resources, serialized commands, and pure presentation, while promoting the Room-specific Thread panel into a shared conversation boundary.
- Moved application Tours and guide-center navigation out of shared Sidebar UI, and split the wide Sidebar into an app-navigation controller, pure projection, resize behavior, and presentation views.
- Removed the remaining base-layer dependencies on Features by relocating the app shell and Agent Options dialog to their owners and centralizing external-session protocol and labels under `lib/conversation`.
- Unified sidebar and chat-notification bootstrap data behind one queued Home directory resource, and split notification projection, browser effects, and Room WebSocket protocol into dedicated modules.
- Consolidated Home chat and contact sidebar code into one domain, separating Room/DM catalog projection, unread aggregation, directory subscriptions, Room commands, and presentation-only panels.
- Split Personal settings into a scoped profile resource and command controller, data-driven password validation, and presentation-only profile, password, and token-usage sections.
- Moved General settings sections and pure option/preference models into their owning domain, while relocating settings-style UI shared with Operations to an explicit shared layer.
- Replaced the global chat sidebar with a shared, grouped settings navigation while the settings route is active, removed the duplicate settings header tabs, reused the same navigation in the standalone desktop settings window, and moved role-gated operations into the settings management group.
- Reorganized Markdown rendering into core, streaming, workspace, and Mermaid domains, separating link normalization, summary components, SVG rendering, and preview interaction state.
- Split message item rendering into content, Assistant, and User domains, centralized structured tool/result projection, and removed an incomplete custom memo comparator that could retain stale action callbacks.
- Split document preview loading, DOM normalization, and presentation into dedicated modules, while unifying Office preview downloads and payload-size validation behind one resource boundary.
- Split onboarding tour overlays into target/card observation, position strategies, sticker selection, and card presentation, with target highlights now tracking element resize as well as viewport movement.
- Split subagent task inspection into scoped list/thread resources, a serialized command controller, and a presentation-only thread view.
- Split the DM chat panel into entry, model, and view layers, and unified DM/Room session orchestration and conversation surface layout behind shared controllers.
- Consolidated conversation timeline grouping and Room Agent round projection into their domain models, removing the shared conversation utility catch-all.
- Moved long-term memory fully out of the product backend and into the SDK subprocess: removed the legacy memory engine, configuration, HTTP/CLI contracts, entry-management surfaces, and product-side DM/Room injection. The runtime now owns file-based recall and storage through each Agent workspace's `MEMORY.md` index and `memory/` topic directory, while Nexus only projects and edits the underlying workspace files.
- Unified Room and DM auxiliary panels on one shared width policy, removed per-surface headers, and moved panel dismissal onto the active surface tab beside its label.

### Fixed

- Prevented Composer Enter and send actions from replacing an active permission request, and stopped ordinary chat sends from clearing permission state locally.
- Kept Room Agent slots visible without empty preparing or replying labels, and prevented pending client request IDs from being rendered as canonical rounds while chat ACKs are still in flight.
- Restored Select menu defaults when callers pass optional presentation props as `undefined`, preventing Contacts and other Agent identity views from failing during render.
- Grouped Room guidance consumed by a running Goal into the Goal's reply round, kept the model's final response after the user's interjection, removed its superseded round from navigation, and cleared stale Agent reply placeholders from authoritative Room subscription snapshots.
- Made Room user input deterministically outrank Goal and Loop continuations, persisted guide state in the public timeline, and converted guidance that misses the final tool hook back into a normal queued turn.
- Accepted Skill zip archives whose entries use Windows backslash separators, allowing preview and import to find nested `SKILL.md` files.
- Computed group-chat sidebar previews from the room's shared history instead of an arbitrary member session, surfaced conversation-level activity on room sessions so the newest topic sorts first, and made equal-timestamp session ordering deterministic.
- Restored conversation task-strip updates for runtimes using `TaskCreate`, `TaskList`, and `TaskUpdate`, including structured history replay and legacy text fallback.
- Removed unused Heartbeat requests and dead automation controller surfaces from the scheduled-task page, stopped task toggles from issuing duplicate list refreshes, routed mutation notifications through each task's owning Agent, and propagated failed history commands instead of reporting false success.
- Prevented stale Home Hero font and resize rebuilds from starting duplicate animation loops, and kept scheduled-task picker overlays within the available viewport.
- Prevented stale Room context refreshes, snapshot setters, and mutation results from crossing route Room boundaries.
- Redirected immediately from rooms confirmed deleted by the server instead of racing a stale page snapshot against a second refresh.
- Refreshed authoritative Room context after failed mutations, while reducing multi-member management from one refresh per write to one refresh per submission.
- Reset chat render errors when the active conversation identity changes and localized the shared desktop/mobile fallback view.
- Prevented stale Workspace file refreshes and mutation completions from crossing Agent boundaries, and consumed one-shot file-open Agent signals in both DM and Room views.
- Unified TodoWrite plans and runtime task events through one chronological round projection, preventing message-role-specific task lists and repeated full-message scans.
- Ensured incoming message snapshots remain authoritative even when the existing in-memory collection already contains duplicate message IDs, and ignored invalid negative stream block indexes.
- Prevented queued WebSocket control-message flushes from looping after send failures, reset reconnect budgets for explicit recovery, discarded stale queues on intentional disconnect, and rebuilt sockets whose OPEN transport can no longer send.
- Prevented stale Agent memory snapshots, document loads, and save completions from crossing Agent or file boundaries, while preserving newer edits when a save finishes.
- Prevented delayed Agent name checks and save completions from updating another Agent or closing an editor whose draft changed while the request was running.
- Prevented unchanged Home directory snapshots from repeatedly resubscribing every Room notification channel, and removed duplicate bootstrap refresh listeners from the notification path.
- Prevented stale subagent list, transcript, and command completions from crossing source/task boundaries, and stopped background polling from hiding command failures.
- Removed unreachable Room and Launcher Agent-option dialogs, their delete flow, empty conversation-selection callback, and unused member-selection state.
- Preserved terminal Room slot states in the shared Agent round projection instead of treating a completed slot without messages as pending.
- Fixed conversation navigation so stale round-window requests and removed navigation targets cannot mutate or block the active session after switching conversations.
- Fixed visible conversation window loading to reject stale session completions, handle rejected requests, and retry transient failures without permanently suppressing a round.
- Fixed Goal panels so stale refresh responses cannot replace a newer session or completed mutation, and serialized edit, pause, resume, and clear commands behind one controller.
- Fixed scheduled-task editing to preserve future single-run times, load only the resources required by the active execution mode, and enforce Room member-bound execution consistently in the form and payload.
- Fixed IM pairing Agent reassignment to send the backend `agent_id` field, serialized channel/pairing mutations, and rejected stale list responses so completed writes cannot be overwritten by older directory snapshots.
- Hid redundant Agent/Room names and the DM badge from desktop chat headers while the sidebar is expanded, restored names when it is collapsed, and removed the persistent processed-duration label below the conversation navigator ruler.
- Changed chat sidebar subtitles to show the latest visible reply from each Room's newest session, rendered as compact single-line Markdown instead of repeating the conversation title or exposing raw Markdown markers.
- Preserved subagent thread identity and progress metadata across SDK events, routed Room task controls to the owning Agent runtime, and projected Claude Code child transcript symlinks as full conversation threads instead of raw JSONL output.
- Unified conversation, capability, settings, operations, and contact headers on the shared fixed 52px single-row layout, preserving core tabs in narrow containers and restoring aligned sidebar separators.
- Moved conversation task progress out of the fixed header into a compact, low-radius process list that defaults to a minimal collapsed control and expands on demand.
- Disabled reasoning on `anthropic_messages` title-generation requests and raised the title token budget to 1024, so always-thinking models (e.g. Kimi) no longer exhaust the output cap before emitting the title and leave conversations untitled.
- Kept a round's user message visible in the conversation timeline even when the assistant reply is blank (failed or tool-only rounds), instead of hiding the whole round and swallowing the user's own message.
- Removed old DM rewrite rounds from the SDK transcript, Nexus overlay history, and the active frontend timeline before regenerating the replacement answer.
- Applied agent permission mode changes to active DM and Room runtimes immediately without restarting the SDK session.
- Preserved SDK process wait errors when a DM runtime message stream closes before a terminal result, so round failure logs include the underlying runtime exit reason.
- Cleared stale macOS and Windows desktop WebView caches after app version/build changes and stopped serving HTML fallbacks for missing asset chunks, preventing updated desktop apps from rendering the error screen with stale web resources.
- Added a manual desktop menu action to clear WebView caches and reload the current interface when troubleshooting stale web assets.
- Sized macOS and Windows desktop launch windows against the current screen work area so packaged apps fit smaller displays instead of opening at a fixed resolution.
- Restored chat feed auto-follow when switching between conversations or thread panels with the same message count, and moved the smaller, less transparent floating scroll-to-latest button to the center of the chat feed.
- Made the conversation navigator's active-round status hide based on the chat container width instead of viewport width, preventing overlap when the workspace panel is open.
- Lowered the conversation navigator ruler's top anchor inside the chat container, preventing the first tick and preview from sitting under the header on shorter MacBook screens.
- Stopped generic markdown document paths from being treated as Agent workspace artifacts unless they resolve in the workspace file tree, preventing non-workspace links from opening a 404 workspace preview.

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
