# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added opt-in Linux runtime isolation for both nxs and Claude Code: stable per-owner OS UID/private GID mappings, setgid/default POSIX ACLs, explicit shared-project read/write grants, a root-owned launcher, mandatory PreToolUse path policy, environment scrubbing, and a final Landlock filesystem boundary. Direct `nexusctl` calls are rejected early by policy, and the packaged container denies CLI execution to runtime UIDs until a scoped host broker is available.
- Added an owner-scoped shared-project API, an Operations UI for project creation and member ACL management, and directory-fd-confined host file access for workspace files, transcripts, runtime artifacts, preferences, and user Skill registries. Project membership changes now cancel the affected owner's active rounds and recycle all hot runtimes so stale project GIDs are not reused.
- Added opt-in Linux cgroup v2 owner process-tree reaping: runtime processes inherit a root-owned per-user cgroup, and permission revocation or final session close can call `cgroup.kill` through the trusted launcher.

### Changed

- Merged every responsive desktop Header into the native title-bar plane: macOS keeps native traffic lights, balances the sidebar wordmark between their reserved geometry and the collapse action, aligns Header content to their horizontal center, keeps the collapsed-sidebar control beside them outside the clipped rail, starts the collapsed rail divider below the title bar, and compacts the complete sidebar wordmark at its 264px minimum width; Web centers the collapsed control on its 48px Dock and removes the desktop-only empty `/app` drag Header. Click versus native window drag is arbitrated across tabs, buttons, links, menus, and Header backgrounds with a 4pt gesture threshold while preserving editable controls. Launcher separates its Web and desktop brand geometry: the proportional spotlight row stays `139×16px` on Web and contracts to `104×12px` in the host, while the Striper wordmark keeps its native Web spacing and gains `0.1em` desktop tracking without synthetic weight; Launcher and desktop Home expose only their dedicated top Headers instead of turning the content canvas into a drag surface, and Windows combines WPF WindowChrome caption controls with WebView2 native draggable regions.
- Unified the Create Agent and Create Room dialogs around the compact neutral dialog language: smaller identity anchors, quiet selected navigation/member rows, consistent fields, and fixed action footers; Agent creation now loads its behavior template from the backend and persists the edited Markdown directly as the new workspace's `AGENTS.md`.
- Made the fixed new-Session action easier to discover with a restrained neutral surface, and unified Session/workspace tab close controls behind the same size, radius, hover, and focus behavior.
- Tightened Agent detail identity controls with a smaller inline avatar, neutral avatar and vibe-tag treatments, shared field-label rhythm, a full-width 36px tag-entry row aligned with the name field, and same-scale ghost collaboration actions; removed the redundant desktop back-to-agent-list action and its unused navigation projection.
- Rebuilt the desktop sidebar as one persistent shell: its compact 48px navigation Dock now keeps 32px avatar-scale icon surfaces and identical tab geometry in expanded and 52px collapsed states, while the directory, brand, footer actions, and shell width follow a restrained 300ms damped layout transition instead of swapping two incompatible rails.
- Moved sign-out from the sidebar brand header to the footer's isolated trailing position, kept it reachable at the bottom of the collapsed rail, and strengthened the shared neutral sidebar selection surface for clearer current-location scanning.
- Standardized inactive, hover, pressed, expanded, and active states across History, navigation, settings, capability rows, shared lists, menus, dialogs, and ghost buttons: current rows now use a calm neutral fill with stronger text instead of blue markers, borders, shadows, or redundant badges, while Nexus blue remains reserved for actions, form selection, links, runtime signals, and keyboard focus.
- Flattened Session navigation and capability headers, moved the six-items-per-page conversation History panel to the Session strip's single leading entry, removed the duplicate overview menu, and unified dialogs, dropdowns, action menus, date pickers, avatar pickers, and mention suggestions behind one theme-aware overlay surface and shared menu-state recipe.
- Removed redundant count badges from capability workspace headers and deleted the unused Skill and Connector count plumbing behind them.
- Restored KingHwa Old Song for Chinese conversation prose while keeping Latin text, navigation, controls, code, and workspace content on their dedicated fonts, and tightened the Composer's bottom spacing around the new runtime credit line.
- Replaced 490 arbitrary `text-[Npx]` classes across 169 files with the semantic font-size scale (adding a `compact` 12px step), then swept the remainder: snapped half-step sizes (10.5/11.5/12.5/13.5/9.5px) and 18/20/24px titles onto the scale, capped all remaining `font-bold` usage at `font-semibold` (600) including prose `strong`, unified ad-hoc menu and popover shadows/backdrops on `--surface-popover-shadow` and `--dialog-backdrop-color`, routed the login pages through semantic color tokens, and removed 173 unused i18n keys from each locale catalog.
- Reworked the conversation Composer to the Claude-style layout: a wider 880px lane, a text-only input row without inline shortcut hints, send/stop actions moved into the bottom tool row, and the "Power By NXS" runtime credit centered beneath the shell instead of occupying it.
- Lightened workspace surface headers across pages: session-tab trays now use a faint neutral wash with hairline separators, and view tabs plus header tools sit containerless with hover-only backgrounds instead of bordered, shadowed capsules.
- Rewrote `design.md` as a lean design-system contract: dropped the one-off migration plan and duplicated legacy-language lists, adopted the implemented 4–24px radius ladder with a 20px Composer corner, and documented the unified blue `--primary` semantics across all three themes.

- Moved the Nexus main agent into the chat directory as an undeletable pinned DM, provisioned that default chat during bootstrap, and removed the dedicated Dock action, Focus sidebar, temporary system selection state, and obsolete add-contact-first empty state.
- Aligned User message header-to-body and message-tail spacing with Assistant replies across desktop and compact conversation layouts.
- Reshaped the conversation Composer into a shorter, taller Claude-style focus surface with a 20px radius, a restrained 800px desktop lane, more vertical writing space, and an undivided action footer.
- Recast the light and sunny application shell around a `#f9f9f7` neutral canvas, grayscale navigation and content surfaces with a single continuous sidebar edge hairline, Nexus-blue primary actions and active focus states, neutral secondary tools, semantic red/green/amber feedback, and Chrome-style Session tabs and header tools that use restrained 8–12px corners, fill, hairline contrast, and short separators instead of pill silhouettes or active-tab shadows, while preserving the Home ASCII scene's existing palette.
- Replaced the application shell's full-height and full-width divider lines with theme-aware tonal navigation, directory, and workspace materials plus broad low-contrast edge shadows across light, dark, and rain backgrounds.
- Moved persistent state under `.nexus/app` and `users/<owner>/`, added idempotent startup migration for legacy `.nexus` data, and injects the same owner-scoped runtime root into nxs and Claude Code.
- Authenticated deployments now require archive upload for local Skill imports instead of accepting arbitrary host `local_path` values.
- Deepened only the light theme's warm ambient page background while preserving the existing blue-gray surfaces, controls, borders, text, and dark/rain themes.
- Kept Room conversation tabs in stable creation order while restoring each Room's last explicitly active tab when users return; explicit Conversation URLs and unread targets continue to take precedence.

### Fixed

- Restored the Windows desktop minimize, maximize/restore, and close controls in the WPF host, and restored four-edge window resizing by clipping the WebView2 child HWND away from the caption controls and `WindowChrome`'s 6px resize boundary.
- Moved the Windows desktop startup timeline out of the legacy `~/.nexus/logs` path and into the documented `~/.nexus/app/logs` host-data root.
- Kept the conversation task summary at a stable height and moved its expanded progress list into an anchored popover, so opening task details no longer pushes the message canvas downward.
- Added syntax highlighting to recognized workspace source files by reusing the chat code renderer's theme-aware semantic palette, while keeping unknown plain-text files uncolored and preserving the workspace preview chrome.
- Kept short Markdown documents top-aligned inside full-height workspace previews so unused viewport height remains after the content instead of stretching headings, paragraphs, and list rows apart.
- Taught `tailwind-merge` the custom `text-2xs`/`text-compact`/`text-md` font-size steps so conflicting size classes in `cn()` dedupe by class order instead of leaking through and resolving by stylesheet source order.
- Registered the semantic font-size scale under Tailwind v4's actual `--text-*` theme namespace (it was authored as `--font-size-*`, which v4 treats as font-family): custom steps like `text-compact`/`text-2xs`/`text-md` never generated utilities and `text-xs`/`text-sm`/`text-base` silently fell back to larger Tailwind defaults, inflating dense UI text across the sidebar, tabs, and headers.
- Quieted the Composer's default state: the submit control now falls back to a borderless ghost style until a message can actually be sent, instead of always showing a filled tonal button.
- Aligned content typography with the Claude-referenced restrained weight scale: conversation and workspace Markdown headings, table headers, and dialog titles now cap at 600 (dialog labels at 500), removing the 700/900 weights.
- Anchored the light theme's `--primary` token to the Nexus blue `#5b72ff` so active selections, focus rings, links, inline code ink, and inline progress render in the brand accent instead of near-black, matching the dark and rain themes.
- Defined the previously missing `--shadow-color`, `--surface-muted-background`, and `--danger-text-color` tokens consumed by avatar pickers, provider settings, and runtime settings, restoring backgrounds and hover shadows that silently failed to render; removed the dead `--glow-*`, `--surface-progress-*`, `--surface-inset-shadow`, and `--input-shell-shadow` tokens.
- Flattened the sidebar search action into a borderless ghost icon button, stopped filling the active rail icon with the primary color so rail icons stay monochrome, and aligned the User message bubble and xl avatars with the 12px content radius.
- Matched active workspace tabs to their enclosing pill geometry, standardized sidebar selections on a clear neutral-gray surface without floating shadows, softened Session emphasis, and removed internal divider lines from header controls and inactive Session tabs.
- Unified sidebar contact rows and management-directory Agent cards behind the same routed detail editor, removing the duplicate existing-Agent dialog path.
- Removed the hard horizontal divider above inline Agent actions so the profile surface remains visually continuous.
- Flattened active chat-directory rows into a restrained theme-aware warm gray without the raised card border or shadow, keeping the current conversation legible while visually attached to the sidebar.
- Unified active Session and workspace-view tabs behind one shared border, background, and shadow recipe while keeping each host's intended geometry; workspace tabs, History, Members, and overflow controls now follow the surrounding capsule's inset curve, and active-tab close controls remain clickable above the surface.
- Aligned in-progress Room Agent cards with completed assistant replies by removing the nested recentering lane and reusing the shared responsive message baseline.
- Unified AskUserQuestion cards, options, custom answers, and submission controls with the warm layered surface and elevation system used by the rest of the conversation UI.
- Unified surface search fields behind one shared warm input treatment so capability filters, sidebars, catalogs, and other search entry points no longer carry page-specific borders or backgrounds.
- Enlarged page-level workspace tab capsules with a more comfortable control height, icon scale, and horizontal rhythm while keeping Session headers and compact layouts dense.
- Strengthened the active About, Workspace, and Subagents header tabs with the same complete edge and unclipped warm elevation used by active Session tabs.
- Added consistent spacing between the Identity, Tools, and Skills navigation blocks in the Agent editor across desktop and constrained layouts.
- Preserved Claude Code subagent transcript projection through its runtime-managed output symlink using a double-confined read path, and restored session metadata writes when a valid Agent workspace has not been created yet.
- Fixed the Contacts sidebar add button navigating to an inert management query on desktop; it now opens the shared Agent creation editor directly and preserves the same flow in the phone layout.
- Matched the Session-strip shadow tray to the Session tag radius and changed its soft lift to follow the rounded contour, eliminating rectangular shadow corners at either end of wide or compressed tab rows.
- Reflowed the inline Agent identity overview into a balanced two-row responsive grid so profile and vibe fields share the first row, model selection fills the second row, and the profile description no longer starts below an unexplained desktop whitespace band.
- Replaced bright white utility and catalog icon buttons with one theme-aware warm control surface across sidebar search fields and actions, Agent creation, workspace headers, and the conversation scroll control; renamed the Contacts directory surface to Agent Management and removed its redundant `Agents / AGENTS` heading so navigation and page responsibilities remain distinct.
- Unified workspace view, history, Room-member, and overflow actions inside one warm segmented browser-style capsule while preserving their established order, retained a separate Session-action group, aligned the structure in Agent details, centered controls against the usable header height, and gave identity avatars a defined two-level lift; active Session and sidebar conversation rows now use a complete fine edge plus appropriately scaled soft lift while inactive conversations remain fully transparent and borderless.
- Unified DM and Group Room header rhythm with shared identity, conversation, workspace-tool, and collaboration spacing; removed the redundant divider after the already framed conversation controls, aligned view, history, member, and overflow actions to one control-height baseline, and progressively moves About, Workspace, then Subagents into the overflow menu instead of replacing all three with a separate Panels dropdown.
- Kept the state-layout completion marker in the app-owned migration ledger.
- Fixed state-layout migration failing on transient `ENOENT` while an active runtime cleaned up or moved a legacy transcript entry, or while macOS Finder regenerated conflicting `.DS_Store` metadata; Room append-only overlays now merge provable source/target subsets and equivalent timestamp refreshes, while genuine content conflicts remain protected.
- Fixed unauthenticated App/Web requests falling back to an unscoped Agent or automation query; single-user requests now resolve to `__system__`, while explicit maintenance contexts retain their separate cross-owner path.
- Fixed Room slot interruption by removing the global Composer stop action, binding the stop button to the corresponding `agent_round_id`, preserving that identity through streaming placeholders, and projecting one monotonic stopped slot instead of a `Request stopped` message plus a duplicate empty card.
- Isolated DM and Room input-queue replay by execution scope and prevented Room subscription recovery from dispatching DM queue items through a Room runtime, so a DM resume cannot be reused by a different Room configuration.
- Fixed desktop local profiles being assigned an implicit Free subscription after saving an avatar, which exposed the server-only subscription UI and incorrectly enforced its monthly quota in the App.
- Unified the conversation-header corner hierarchy, removed the washed-out tab-strip and active-tab fills in favor of the ambient background, placed 36px Session tabs inside a roomier 40px transparent shadow tray with a precise 5px inset at both ends alongside the matching create/overview capsule, and derived overflow controls from stable width constraints so the overview arrow no longer flashes while a new Session is created.
- Removed the narrow blue active marker from shared directory rows so selected capabilities and conversations rely on the shared active surface and stronger content hierarchy.
- Closed the 6px divider gaps at the expanded sidebar brand row and utility footer so their horizontal rules meet the workspace and sidebar boundaries while preserving the sidebar body's outer breathing space.
- Increased the shared desktop workspace header from 52px to 60px with proportionally taller conversation tabs and actions, and gave the Panchang NEXUS wordmark a restrained weighted, shaded, and softly embossed treatment instead of leaving the ultra-wide header visually flat.
- Replaced broad blue-gray selection fills across conversation tabs, primary chat navigation, sidebar rows, menus, settings navigation, and the mobile conversation switcher with a shared theme-aware active surface and stronger text; the light theme now uses a low-saturation warm-ivory surface just above the ambient background with a fine warm-gray edge and soft lift, leaving the brand color only on compact state indicators.
- Restored a low-contrast, theme-aware shadow tray beneath the conversation tab strip while keeping the overflow and create controls transparent and preserving the stronger raised state for the active Session.
- Expanded the centered conversation rail progressively on wide and ultra-wide desktops, keeping messages, status surfaces, and the Composer on one axis while retaining a narrower readable limit for assistant prose and leaving compact layouts unchanged.
- Combined the conversation overview and new-session actions into one neutral, transparent icon-only pill with width-aware spacing; overflowing tabs now recalculate complete visible slots and snap scrolling to tab boundaries while keeping emphasis on the active Session tab.
- Increased chat sidebar row height and vertical breathing room across DM and Room entries, with matching loading placeholders.
- Reworked the personal profile into a balanced full-width identity card with the same large-avatar dropdown used by Agent and Room editors, evenly filled role/auth details, and an explicit locked-avatar state.
- Removed the duplicate desktop Room member entry from the overflow menu while preserving it there whenever the dedicated member control collapses on narrower screens.
- Hid the conversation scroll-to-latest control when an empty or short message viewport has no real scrollable overflow.
- Toned down the Room collage avatar shell and member control with theme-aware ambient surfaces and quieter separators instead of bright white group-identity backgrounds.
- Replaced the conversation overview's white paper background with a theme-aware translucent ambient panel while preserving the selected conversation accent.
- Gave crowded conversation tabs slightly wider readable minimums, separated the overflow rail from the tab edge, and kept it hidden until hover while reserving the stronger accent for active dragging or keyboard focus.
- Replaced the sidebar create-room button's fractionally scaled plus glyph with a pixel-aligned equal-axis mark so its cross remains visually centered at desktop and constrained widths.
- Removed the leftover 6px desktop stage top inset so the sidebar, workspace, Launcher, and lightweight desktop surfaces sit flush with the window top while preserving side and bottom breathing space.
- Raised the regular desktop conversation composer slightly with a 16px bottom breathing area while preserving its existing size and the phone safe-area layout.
- Kept the expanded sidebar brand divider on the same shared 52px header baseline as the workspace header at constrained desktop widths.
- Removed the arbitrary five-tab ceiling from wide Room headers, made Safari-style tabs progressively share and shrink across the available track, added a lightly elevated active state plus direct trackpad, mouse-drag, and scrollbar navigation after labels reach their readable minimum, and only reveal the overview control when the tab strip actually overflows.
- Blended the narrow-window conversation switcher into the warm ambient surface with one translucent material across its title and list areas instead of rendering a cool gray or opaque paper panel.
- Made crowded Room conversation headers easier to scan with bounded browser-style tab widths, individually framed active and inactive states, and quieter close actions on background tabs.
- Removed the empty flex-growth band between Agent identity profile fields and their secondary settings in narrow layouts, and aligned Room settings with the same content-driven grouping and click-to-expand avatar picker while preserving desktop column allocation.
- Kept Group Room member management reachable at every width: the phone-layout overflow menu now includes Members, desktop and mobile share one management-dialog adapter, and the narrow-window dialog stacks settings, members, and skills in a content-sized single column with bounded scrolling instead of stretching to the full viewport.
- Moved Chat, Contacts, and Capabilities into a labeled vertical dock in both the expanded desktop sidebar and phone directories, while preserving the icon-only collapsed desktop rail, retaining the upstream transparent patterned sidebar surface across navigation states, and moving phone-directory system actions into the bottom of the same dock.
- Gave the phone and narrow-window composer a centered `720px` width ceiling, larger edge and safe-area insets, and a slightly shorter idle input row instead of letting it fill the entire bottom edge.
- Reworked Agent option density: avatar selection now opens a large five-column picker from the current avatar instead of using a draggable horizontal strip, while identity fields, tags, model controls, permission choices, tool rows, and skill cards use roomier responsive spacing and touch targets.
- Added an application-level phone layout: chat, contacts, and capabilities now become full-screen primary directories, Room and detail pages use explicit back navigation, the conversation switcher expands from its header trigger using the same directory row language, Agent management uses compact single-column cards only below the desktop breakpoint while preserving its comfort cards at normal widths, dense headers collapse into labeled menus, and Agent editing now uses a content-sized desktop dialog, separated top navigation in constrained windows, and a single-column near-full-screen mobile form instead of squeezing desktop sidebars and controls. The macOS and Windows shells keep the `1280×820` initial size while allowing a `360×520` regular minimum and a `320×480` tiny-work-area fallback.

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
