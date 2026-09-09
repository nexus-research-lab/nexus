# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]


### Added

- Translate scheduled board column headings without changing task grouping or order.

- Use a localized task label instead of exposing the internal executor Agent ID when a scheduled task has no source name.

- Localize scheduled-task card action labels, accessible names and mutation-protection hints.

- Localize scheduled-task permission actions in cards and details while preserving pending and unknown-result protection.

- Give scheduled-task titles a shared editable-title input role and use shared supporting typography for confirmation summaries.

- Preserve the redesigned scheduled-task editor while separating destination projection and reusing shared panel and plain-select surfaces.

- Associate agent skill switches with their purpose descriptions and wrap long unbroken card text.

- Remove unused workspace surface header modes while retaining existing product layouts.

- Keep onboarding open when Escape is consumed by another control or an input method.

- Make Mermaid source regions keyboard-accessible while preserving native scrolling and source formatting.

- Keep appearance number drafts intact during IME confirmation and match adjustment feedback to the current value.

- Preserve browser zoom gestures over conversation tabs and release wheel events at scroll boundaries.

- Reuse the shared icon button for conversation pinning, preserving independent tab actions.

- Anchor global feedback at the top-right with a small safe-area-aware margin.

- Name onboarding card regions and let long guidance and navigation controls wrap.

- Avoid dangling tooltip descriptions when labels are empty while preserving existing accessibility hints.

- Place pairing authorization badges next to the external contact name.

- Localize Markdown workspace-file hover hints while preserving exact-path keyboard actions.

- Make Mermaid preview canvases keyboard-focusable and give each dialog its own accessible title identity.

- Let management page headers grow and wrap actions when their content exceeds the standard height.

- Keep the main Nexus chat pinned and protected as runtime identity loads, and refresh chat ordering on durable user activity and round changes.

- Collapse settings search to an icon until activated, and keep process disclosure text muted on hover.

- Place generated-file summaries after assistant replies instead of beneath process headers.

- Unify streaming code and syntax-loading placeholders in the Markdown code renderer.

- Allow model capability labels to wrap and activate their switch when clicked.

- Remove manual font entry and its helper copy; keep font selection in the dropdown.

- Clarify sidebar conversation name hierarchy and show font fallback guidance without truncation.

- Show the full settings back label and refine Nexus wordmark proportions while preserving its original colors.

- Keep conversation process disclosure text regular and muted when expanded.

- Compact model configuration with header identity, two-column capabilities and
  advanced JSON disclosure that stays open for existing custom options.

- Restore a consistent indent beneath settings navigation group headings.

- Use title-case Nexus branding and expand the compact settings search on focus or query.

- Put settings Back and Search controls on one row with a named compact back button.

- Refine the fixed sidebar wordmark to 22px with balanced weight and tighter spacing.
- Align community-search Enter behavior with disabled and IME input boundaries.

- Show typography limits and warn before clamping out-of-range values on blur.
- Keep the sidebar NEXUS wordmark left-aligned with fixed size and letter spacing.

- Consolidate guides in the compact account menu, removing duplicate Skills and
  standalone help actions while retaining first-visit onboarding.

- Replace the duplicate community-search magnifier with a text Search action.
- Localize code-group/loading labels and remove decorative code shells from Tab order.

- Use the shared icon button for code copying, restoring consistent keyboard focus
  and accessible success feedback while removing duplicate code-button CSS.

- Reduce compact shared status text to regular supporting typography, with a smaller
  icon frame and spacing for loading and empty states.

- Prevent overlapping desktop update-hint reads from replacing newer version hints.

- Remove the unused internal category-prefix rendering branch from shared select triggers.

- Remove obsolete bilingual category-prefix strings after shared filter consolidation.

- Simplify shared directory filters to the selected value and chevron, removing
  repeated category prefixes while retaining accessible names and explicit all-category labels.

- Compact sidebar footer actions: settings sits beside help; local mode hides the
  account bar, while authenticated Web sessions retain an avatar logout menu.

- Add an explicit, cancellable Team load retry with duplicate-request protection;
  retrying a read never resends a chat message.

- Restore conversational administrator user management through Control-backed `nexuscfg members`, with human approval, secure password entry, profile/role updates, and deployment access revocation. Remove obsolete user/auth CLI instructions.

### Changed

- Restore the uppercase, thin Panchang NEXUS sidebar wordmark with wide letter spacing
  and its original theme-aware graphite gradient and shadows.

- Aligned WorkGraph skills and command guidance with the actual inspect recovery,
  XML context and Draft version operations. Removed fictional coordination-mode
  instructions and repetitive edit confirmations while retaining save confirmation.
  Preserved domain rejection codes, enabled editor clarification, and prevented
  historical graph reads from replacing current round coordination authority.

- Aligned Execution and Goal skills with fixed inspect entry corrections, including
  recovery-action routing and the separate invoke path for WorkGraph Draft queries.

- Preserve Team history reading position when messages arrive and reuse the
  shared return-to-latest action to resume following.

- Align Team message typography with shared styles, allow author rows to wrap
  and preserve complete Unicode initials.

- Keep scheduled target picker content scrollable in short windows and describe
  the current target accessibly on each trigger.

- Align scheduled target pickers with shared popover styling and return boundary
  Tab navigation to the surrounding form.

- Show font catalog loading, allow empty-catalog manual input and retry on reopening,
  and ignore stale font responses after the picker closes.

- Refined the execution activity dock with shared 28px avatars, fewer nested
  styling overrides and a shorter divider while preserving navigation and hit areas.

- Search settings item names and descriptions in addition to navigation labels, listing matching items under their accessible module.

- Add settings navigation search by section or group name and soften the back-to-workspace label weight.

- Align settings navigation icons and the back action with section headings using a shared 8px content inset.

- Narrow the primary navigation rail by 8px to give the adjacent directory more room, keeping pinned items centered.

- Distribute the sidebar NEXUS wordmark across the available header width with balanced spacing and a reserved collapse action.

- Refine the sidebar account menu with an avatar header, grouped actions, regular-weight account text, and width aligned to the footer.

- Compact the sidebar account footer to 48px and add online documentation to the guide center.

- Restore the unified glass Launcher composer with an inset mascot, larger recent-entry markers, and a separate centered handoff row.

- Group sidebar settings and sign-out under the account menu, with a dedicated help action at the bottom right.

### Changed

- Simplify automation creation with a right-side editor alongside the unchanged board, searchable two-column execution and recipient pickers with Agent/group-chat filters, clearly labeled optional task titles with an inline close button and no empty editor header, generated task names, live confirmation summaries and concrete inherited permission labels, explicit result delivery, safe unique-session defaults, and secondary options grouped under advanced settings; retain the existing task board.

- Expose “Approve for me” for Claude and preserve native auto mode through session and task configuration; confirm activation in the bridge instead of silently using manual approval.

### Fixed

- Prevent Team messages from being sent while confirming IME composition and
  announce loading failures without displaying an empty conversation.

- Constrain anchored popover dimensions to the viewport even when a preferred
  minimum height is larger than the available window.

- Keep loaded scheduled-task destinations usable during partial failures, avoid
  misleading empty results and retry only failed resources that are not loading.

- Allow manual font selection after desktop font catalog loading fails, preserving
  the current custom font.

- Avoid requesting Select opening resources when arrow keys have no available option.
- Clarify fixed inspect operations in command contracts and return the exact inspect call when agents incorrectly invoke them, preventing misleading operation-registration retries.

- Navigate settings search results to the matching field, including repeated clicks within the same section and asynchronously loaded content.

- Center sidebar navigation icons and labels across the full visible rail, including the shell's leading inset.

- Fix completed legacy queued runs being restored as active task occupancy; reconcile proven finished remnants during scheduler audits, reject stale runtime snapshots, and show unknown start times honestly.

- Store each Team Conversation and Relay Message once per deployment while keeping only per-owner recovery cursors, including migration of existing duplicated projections.

- Keep Team local-projection cursors behind failed writes, recover Relay stream-epoch changes through a typed WebSocket reset, and replay committed sends through difference before advancing the browser cursor.

- Accept automatic review in DM/Room session settings and classify validation failures as not applied. Show save failures in a dismissible dialog without leaving an error banner or refresh action.

### Changed

- Replace the selectable automatic-edit mode with “Approve for me”, backed by native SDK review, and preserve review reasons in conversations and scheduled-task approvals. New Agents default to automatic review; existing modes remain unchanged.
- Keep automatic review hidden for Claude until its native mode is integrated; project saved automatic-review presets to manual approval in the current Claude adapter.
- Move Agent-specific permissions into collapsed advanced settings, explain their relationship to system defaults, and remove per-tool authorization switches. Preauthorize web search and fetch while preserving explicit restrictions.
- Link Agent connector cards to connector settings, retain gray styling for disconnected connectors, and keep activation switches independent.

### Added

- Alert users to pending approvals and questions while Nexus is in the background with a numeric macOS Dock badge (without bouncing) and Windows taskbar flashing; clear attention on return or resolution.

- Added local system-font selection, font size, and line spacing preferences with live preview, row-based appearance controls, and one-click theme and typography reset.

- Added opt-in Nexus Relay configuration, fixed-audience Control user token
  exchange, an independent typed Relay M1 HTTP client, and authenticated
  `/nexus/v1/team/...` browser gateway endpoints with Relay stream-epoch propagation,
  WSS-triggered difference recovery, owner-scoped local projection, and a shared
  General Room in the existing chat UI.

### Fixed

- Fixed LaTeX parenthesis/bracket delimiters and formulas containing blank lines in streamed replies.

- Fixed Room scheduled tasks remaining running after their round completed, and
  restored owner-scoped transcript permission repair for Room history, page
  loading, Agent wakeups, and scheduled result delivery.
### Added

- Improved shared avatar picker keyboard focus, form Tab navigation and disabled/value
  reset behavior; unified profile avatar sizing and localized avatar choice labels.

- Added DM and group session-header regressions covering creation with delayed
  catalog refresh, history opening, tab switching, close/reopen and persisted
  tab/pin restoration through the real page navigation and command handlers.
- Added real Launcher/workbench browser and native UI fixtures with read-only
  HTTP/WebSocket boundaries and a regression that prevents backend forwarding.
- Added an isolated macOS frontend UI check that compiles the current native
  window and WKWebView, verifies trusted input and nested overlay behavior across
  themes, languages and window sizes, and records source hashes and screenshots.
- Added a contract gate that keeps Windows native semantic colors aligned with
  their Web light-theme token owners, including native-theme CI path coverage.
- Added a frontend CI gate with import-aware architecture checks, shared UI
  file contracts, and a reproducible browser matrix across three themes,
  two languages, and narrow/desktop widths, with screenshots and failure traces.
- Added a shared split-action Button pattern with independent primary and menu
  commands, keyboard focus, ARIA, tests, and UI Gallery coverage.

### Changed

- Made Agent identity tag fields respond to their actual container width and
  removed redundant field wrappers. Unified editor action sizes and made save
  confirmations fully readable and accessible without changing save commands.
- Consolidated desktop DM and Room header navigation wiring and stabilized Thread
  controls across unrelated page updates. Extended real session-tab and layout
  regressions for panel switching, retained chat input and read-failure recovery.
- Deferred desktop auxiliary page initialization until first use while retaining
  visited pages during panel navigation. The resizable panel now has a localized
  accessible region name, with unchanged file commands and width preferences.
- Aligned Room profile tabs with shared compact navigation. Profile member choices
  now discard removed members and stale Room/owner requests while preserving
  sections across same-Room session changes and ordinary catalog refreshes.
- Improved Room execution controls with consistent 28px buttons, 12px labels,
  Agent-named Thread actions and explicit expanded/busy states. Thread empty
  details now use the shared resource state, including accessible announcements.
- Unified narrow Room auxiliary, Thread and subagent overlays with one modal
  frame, keyboard dismissal, focus return and scroll locking. Improved the
  narrow header's session subtitle readability and complete accessible titles.
- Unified narrow and desktop Room member opening, preventing duplicate catalog
  loads and late dialogs after navigation. Narrow action menus and task/auxiliary
  overlays now clear on session or owner changes while preserving equivalent
  identity refreshes and Room-scoped member editing.
- Unified Agent save feedback and full error details with shared overlays,
  accessible status text and keyboard dismissal. Compact contact actions now
  close when switching Agents and use explicit, named commands.

- Improved shared action menus with readable semantic typography, complete
  wrapping labels and descriptions, content-based popup sizing, consistent
  selected/disabled states and clearer Provider metadata in model menus.

- Unified Select keyboard focus, Tab exit, localized placeholders and full option
  labels; Provider testing now uses an explicit action menu so browsing models
  with arrow keys cannot start a test. Provider names and actions wrap cleanly.

- Compact workspace view selection now uses the shared Select Menu with a
  named current view, keyboard navigation and focus return. Hidden triggers and
  changed selection contexts discard open menus. Removed unused Header slots,
  stale styles, hidden session-header selectors and the unreachable standalone
  Provider header; contact directory returns use an explicit action slot.

- DM, Room and contact headers now use the standard 40px avatar directly,
  removing duplicate avatar frames and shadows. Room member summaries keep
  compact shared avatars and a readable, growing extra-member badge, with one
  accessible count. Loading prevents duplicate opens, and Room/owner changes
  discard pending or open member dialogs without reviving them on return.

- Room Goal owner selection now uses the shared compact Select Menu and the
  same disambiguated names as status and Plan-mode explanations. Missing owners
  remain explicit and must be reselected instead of silently assigning another
  member; creation checks the current roster before sending the host command.
  Room Goal status reads the canonical Goal directly, removing a delayed local
  copy. Session/candidate changes close stale menus while preserving the trigger,
  and Room/DM continuation explanations follow the current interface language.

- Goal controls now localize status, editing and confirmation copy, wrap long
  blocking instructions and identify the action actually in progress. Budget
  editing rejects partial or unsafe numbers instead of changing their value.
  Session/owner/Goal changes discard old drafts and confirmations; changed
  objectives or clear permissions invalidate confirmation without reviving it.
  Stale successful writes retain a read-only recovery action, and editing shows
  recovery feedback once with independent accessible field and dialog names.

- Task summaries now use the shared flat surface and standard small avatars.
  Task details share bounded popover positioning, layered dismissal and keyboard
  focus navigation. Session changes clear old panels, reordered or replaced
  tasks cannot inherit another task's open details, and valid live updates keep
  reading focus. Room task selection recovers from removed members/processes
  without reviving obsolete manual choices; summaries and menus share names.

- Room member selectors now share the compact public avatar, recover failed
  images and distinguish duplicate or unnamed members without exposing IDs.
  Missing selections stay visibly unavailable; stale menus close when their
  candidates or controlled selection change. Task selectors use the full Room
  directory for stable names while preserving their actual task candidates.

- Subagent navigation now respects manual caller/task choices, preserves them
  across catalog reorder and consumes old links when switching conversations.
  Missing targets offer one refresh action. Narrow task panels use shared modal
  focus and layered dismissal, and returning from details restores row focus;
  Thread navigation labels follow the current language.

- Subagent directories now distinguish active, historical and unknown tasks,
  label queued/failed/stopped work and preserve readable tasks after refresh
  failures. Valid observation times use shared localized labels that refresh
  while visible; unknown states cannot clear an unconfirmed stop result.

- Subagent details now use shared avatar sizes, typography and resource states,
  with wrapping controls, accessible pending actions and localized fallback names.
  Thread file actions preserve the resolved source workspace; explicit missing
  scope never falls back to a runtime task ID. File-only replies remain visible,
  and final file deliverables appear once alongside the final response.
- File artifacts now preserve their source workspace through message bodies,
  collapsed processes, WorkGraph history and Room Thread callbacks. Missing
  file identity remains visible with a localized explanation instead of opening
  the selected Agent's workspace; downloads remain available without preview.
  File cards share typography roles and localized collection labels.
- WorkGraph node steps now use readable shared typography, wrap long text and
  offer access to the full list, with counts and rows based on the same normalized
  tasks. Node and run details share localized status/time formatting; unknown
  records no longer display internal IDs, and historical errors use shared feedback.

- Composer Agent activity docks now pair clearer 26px avatars with a lighter
  36px toolbar while retaining 32px click targets. Status uses one corner dot;
  narrow layouts scroll the avatar strip while keeping the WorkGraph entry
  available, with keyboard navigation and exact round links preserved.

- Scheduled task cards and run history now share localized error summaries;
  internal run and delivery details remain available in diagnostics and copied
  reports. History retries share loading behavior, retain usable snapshots after
  transient failures, and keep inaccessible records hidden during recovery.

- Scheduled history feedback now follows the current language and preserves
  warning semantics when an action succeeded but history could not refresh.
  Reopening the same task no longer accepts old command feedback or clears new
  pending actions; synchronous command errors also release local deduplication.
  Removed unused output labels and the history-specific error border recipe.

- Scheduled run history now updates status, duration, dates, action hints and
  confirmation labels with the current language while preserving expanded rows
  and exact confirmation targets. Request busy states remain distinct from
  unconfirmed outcomes; refresh is disabled while loading, and dialog titles use
  the shared instance naming protocol. Invalid history dates degrade gracefully.

- Bound scheduled-run artifact actions to their historical executor instead of
  the task's current Agent. Unproven locations now show a disabled action with
  an explanation, and copied diagnostics distinguish current and historical IDs.
- Shared file-action failure handling across previews, message artifacts and
  run history, with localized feedback isolated by account, source and file.
- Kept scheduled-task Room and session selectors compact and free of internal-ID
  fallbacks. Duplicate names share one numbering rule, and missing selections
  remain visible through catalog refreshes without changing saved bindings.
  Removed redundant parent-name indexes and duplicate session-label formatting.
- Reused shared Badge and Typography styles in select menus, removing the
  private 9px badge. Select triggers now ignore composition and already-handled
  keyboard events so input-method confirmation cannot change a selection.
- Unified Agent selection labels across pairings and scheduled tasks, using
  display-only ordinals for duplicate or missing names without exposing IDs.
  Missing bindings remain visible, and pairing creation no longer silently
  switches to the first Agent when the selected Agent leaves the directory.
- Isolated pairing dialog title and field IDs so multiple instances retain
  correct label associations.

- Removed unused legacy Room round projections and kept timeline grouping
  independent of display names and avatars. Room cards now resolve current names
  at the display boundary while preserving exact Thread and stop targets.
- Updated pairing group headings from the current Agent directory and normalized
  unnamed Agent labels in pairing and skill deployment feedback. Skill deployment
  feedback now belongs to the operation controller instead of the detail page.

- Kept sidebar and private-thread previews compact by replacing parsed equations
  with localized inline labels, while retaining prose, prices, code examples and
  full formula rendering in message bodies.
- Centralized missing Agent/Subagent display names across previews, private
  threads and execution views without changing IDs, navigation or avatar seeds.
  Generic role labels no longer count as real owner names when shortening objectives.

- Restored Agent names in DM and Room session-navigation previews by passing the
  current name directory through the shared panel model. Missing or blank names
  now use a localized generic label instead of exposing internal ID prefixes.

- Added shared model-output formula compatibility for LaTeX bracket delimiters,
  protected formula source from Markdown preprocessing, and kept incomplete or
  invalid formulas readable. Display equations retain blank lines while streaming
  and use keyboard-accessible horizontal scrolling when wider than the content.

- Unified Composer question typography and decision touch targets, isolated radio
  groups and request drafts, and made asynchronous submission survive effect
  replays and request changes without stale completion or duplicate dispatch.

- Consolidated queued-message headers into shared Disclosure, added keyboard
  move actions and full-content descriptions, and limited dragging to its handle.
  Queue commands now reject stale/no-op orders, serialize dispatch, and stop
  idle edge scrolling; transient queue UI resets on Session changes.

- Unified Composer footer metadata and context details with shared typography,
  and consolidated Goal and Connector toggles into accessible checked menu rows.
  Fixed first-click context dismissal, retained keyboard-focused details, and
  kept directory and Session recovery actions aligned with mutation locks.

- Composer files and local directories share removable chips and named actions.
  Image/text previews share one accessible dialog header and scrollable recovery;
  file and Session changes clear obsolete preview state, and blocked directory
  mutations are visibly disabled while reload remains available.
- Spreadsheet previews now preserve source point-to-pixel font sizing and
  vertical alignment, share readable coordinate labels and keyboard-scroll
  styling, and expose ordered virtual rows and merged cells as a read-only table.
  Slide thumbnails reuse the full-size content layout; repeated paragraphs and
  text runs no longer collide during React updates.
- DOCX, XLSX and PPTX previews share file, account and retry scope handling;
  stale reads and parses cannot replace current content, and obsolete slide
  resources are released. DOCX retains its rendering hosts after failure so
  retry can complete; Office loading and recovery surfaces remain scrollable
  in constrained panels without changing document, workbook or slide layout.
- Image and PDF previews share scoped native-media state and reset on file or
  account changes. Image recovery remains scrollable in short panels; PDF gains
  an explicit reload action instead of relying on unavailable iframe errors.
  Streaming HTML shares source-preview styling and uses one commit timer while
  preserving its sandbox, storage shim and immediate final update.
- Large text previews reset pagination when the file or account changes and
  ignore obsolete reads. Chunk navigation retains its controls while loading,
  uses fresh byte offsets and keeps only one chunk. Plain and chunked text share
  source metrics and named keyboard-scrollable viewports; paging controls can
  wrap and read failures remain scrollable in short panels.
- Workspace file actions keep failure feedback scoped to the current file,
  account and latest explicit attempt, and update it when the language changes.
  Text editor read/save recovery states share one compact layout while preserving
  explicit reconciliation and conflict decisions. Header sync metadata uses the
  shared icon sizes and retains its lightweight presentation.
- Workspace directories distinguish a failed initial read from a confirmed
  empty list, with one retry surface. Cached files remain available after a
  refresh failure, and dismissing feedback preserves the failure state. Focused
  previews retain the directory instance and expansion within the same Agent;
  stacked directories shrink with short windows and share one scroll container.
- Desktop file, Thread and auxiliary splitters now support arrow keys and
  Home/End, with visible keyboard focus and announced width ranges. Keyboard
  resizing starts from the effective CSS-limited width and updates the existing
  layout preference; viewport changes preserve that preference. Panel width
  limits and resize keyboard handling have shared owners.
- Auxiliary and workspace file panels share mouse drag cleanup. Resizing ends
  on window blur, hidden documents or a released primary button, and file-list
  dragging stops when switching to stacked or focused preview layouts. Secondary
  mouse buttons no longer start a resize; zero-width containers keep their size.
- Workspace file trees preserve nested expansion after parent collapse and
  directory refresh, and discard preferences for removed directories. File
  rows use shared buttons, named disclosure states and full-path action labels;
  bounded names and indentation keep actions within the panel. Initial file
  loading and empty directories now reuse shared state components.
- Catalog headers, descriptions and footer actions now respect narrow columns;
  removed unused body growth, description reservation and icon tone options.
  Scheduled history uses the shared Badge directly, and list dividers use
  their visible group labels as accessible names. Mixed checkboxes now keep
  DOM and ARIA aligned until the parent accepts the selection.
- Workspace search now uses the shared input directly, with localized default
  text and a reliable accessible name. Room fallback navigation uses shared
  catalog actions; retired its separate action-card wrapper and unused layout
  variant. State messages, recovery actions and catalog content now wrap long
  text within the available width.
- Composer settings menus now close when their Session changes, and permission
  scope menus reset with the request or when actions become unavailable. Failed
  permission delivery preserves entered secrets for retry. DM/Room share model
  selection handling, and long Provider labels leave space for model names.
- Room model menus now keep one options list across wide and narrow layouts,
  preserving focus and scroll position. Shared overlay bounds constrain the
  full panel width, narrow headers count toward the height budget, and DM/Room
  reuse the same model width. Removed duplicate options markup and viewport
  calculations; menus close when their selected Agent Session changes.
- Workspace context menus now use shared pointer/cascade overlay bounds, modal
  dismissal and scrolling; long application lists update in place, and crossing
  a submenu gap keeps it open. Removed file-specific size estimates and private
  positioning/listeners. Menus share total row/separator geometry, and Tab exit
  skips closing parent portals; stale invoking elements dismiss cleanly.
- Action, Room model and Workspace menus now share keyboard traversal and Tab
  exit behavior, skip disabled items and preserve IME input. Cascades support
  keyboard entry and stepwise return without moving focus on hover; Session
  model and desktop file commands keep their original targets. Dialogs and
  menus share one DOM focus order, and Room menu height estimates use shared
  row metrics instead of duplicate constants.
- Mention suggestions now share the common listbox surface, option rows and
  live anchor positioning; keyboard handling respects the current editor and
  modal scope, with active-option accessibility and focus preserved on selection.
  Shared overlay bounds now stay visible when anchors scroll outside the viewport.
- Unified menu row sizing with popup height estimates and removed Mention's
  duplicate positioning constants and one-time anchor snapshots.
- Launcher queries now use the shared input style and keyboard focus treatment;
  the mascot send action keeps its accessible name while busy. Removed unused
  Launcher color variables and redundant recent-entry wrappers, and fixed IME
  confirmation being captured as a Mention selection or query submission.
- Launcher decorations now respect reduced motion from the first render:
  Lottie switches between looping and static lifecycles, while Hero text and
  entry containers use shared CSS motion rules. Removed duplicate playback,
  media-query listeners, unused style props, font measurement and entry timers.
- Shared streaming file previews now use the source editor's text metrics,
  localized logical line counts outside the text, and a motion-aware cursor;
  removed redundant width measurement and runtime style injection.
- QR displays now localize loading and failure feedback, handle image decode
  failures, and discard outdated generation work; skeleton card lists announce
  loading once instead of repeating it for every placeholder.
- Removed unused dismiss behavior from ordinary view/filter selectors and
  centralized their typography; workspace session tab controls remain intact.
- Restored the previous Memory page layout as a whole, including its compact
  search/filter row, directory, document header and split-pane structure, while
  aligning the base background with sibling Agent tabs, retaining document
  surface depth, shared source editing and file recovery behavior.
- Unified workspace file loading and empty states across media, Office and text
  previews, removed duplicate header feedback and unused status copy, and
  localized unsupported-file guidance for browser download and desktop reveal.
- Share source-editor typography, scrolling and keyboard focus across workspace,
  profile and Memory surfaces; associate profile labels with their editor and
  isolate pending save confirmations while preserving newer drafts.
- Make Memory search and type filters readable, consolidate empty/loading states,
  wrap document headings and actions, and preserve navigation on access failures.
  Size conflict comparisons to the document pane instead of the browser window.
- Unify contact communication search and empty-state actions, keep pending friend
  additions reviewable, and bind removal confirmations to their original target.
- Unify Agent identity labels and field density, associate name validation and
  template guidance with their inputs, and let shared Select controls grow for
  long model names. Preserve IME composition in tag drafts, localize removal
  actions, and restore input focus after adding or removing a tag.
- Keep Agent and Room identities readable when avatar images fail; fit initials
  to member mosaics and preserve complete Unicode characters. Share character
  segmentation across avatars, Launcher and text rendering, and remove duplicate
  initials and image-rendering implementations.
- Unify Contacts grid cards and creation/search entries across breakpoints, make
  Agent names and Provider metadata readable, add clear-filter recovery, and
  share the labeled directory filter with capability pages through `UiFilterSelect`.
- Refine Skill import and source management with instance-scoped fields,
  readable source addresses/statuses and import guidance, shared list surfaces,
  technical input typography and explicit busy states. Remove forwarding
  wrappers while retaining Git drafts, stored-credential semantics and commands.
- Improve shared segmented controls with readable density, aligned option
  heights, wrapping labels and keyboard-accessible icon hints. Remove the
  Gallery-only group-icon API and a duplicate settings field wrapper; guard consumers
  against private visual overrides while preserving selection commands.
- Refine scheduled-task forms with instance-scoped field labels, named choice
  groups and shared help text. Align interval controls, give schedule choices
  their own row, and preserve raw instructions, schedules and task routing.
- Align shared Choice sizes and interaction states, reuse primary token colors
  for date/time selections, and preserve native disabled hit targets. Show full
  Agent permission descriptions and isolate repeated tool-permission radio
  groups without changing authorization commands.
- Refine shared checkbox rows with readable compact labels, separately named
  help, wrapping text and native disabled hover handling. Remove a redundant
  Runtime settings wrapper while preserving independent setting updates.
- Make shared prompts and overlay dismissal IME-aware, localize default decision
  actions and hints, and bind prompt errors and descriptions to named fields.
  Keep workspace prompts locked during writes and localize Shopify validation
  without clearing drafts when the interface language changes.
- Refine desktop and mobile conversation history with shared readable metadata,
  row-action visibility, scrollable batch-result feedback and a localized empty
  state. Keep IME candidate keys out of title saving, restore editing focus,
  and give mobile history instances distinct accessible names.
- Unify Composer Loop and WorkGraph picker surfaces and readable metadata,
  focus search on opening, and add keyboard navigation to WorkGraph previews.
  Prevent duplicate Loop starts and late completion from closing a reopened
  picker; remove unused Session plumbing from the owner-wide WorkGraph catalog.
- Localize Connector detail actions and connection guidance, share resource
  states and compact capability rows, and give capability dialogs explicit
  accessible names. Wrap long object names, endpoints and instructions to the
  actual pane width, consolidate OAuth configuration buttons, and remove an
  unsupported fixed token-expiry hint without changing connection commands.
- Give shared interactive list rows a consistent inset keyboard focus ring.
  Unify Connector, custom MCP and Loop row actions, localize Connector card
  labels, and replace private Connector/MCP loading and empty views with shared
  resource states while preserving command targets and recovery behavior.
- Unify six General and Runtime toggle rows with readable, wrapping descriptions
  and setting-specific accessible names. Associate Browser permission risks and
  model-enable hints with their switches, preserve independent saving locks,
  and remove twelve unused English/Chinese toggle labels.
- Bound Provider model and usage dialogs to the shared viewport, keep actions
  visible while their body scrolls, and show full model and agent names. Share
  field sizing and focus handling, retain named busy actions, and consolidate
  repeated capability rows without changing model or deletion commands.
- Adapt Provider configuration columns to the actual detail-pane width, align
  standard field sizes, and present fixed endpoints as read-only rows. Isolate
  field labels per instance while retaining permissions and blur-save callbacks.
- Keep Custom MCP parameter and secret rows stable during edits and removal,
  give each input and delete action a distinct name, and share technical input
  typography and labeled choice groups while preserving masked-secret saves.
- Let Room context details grow to their content within the shared viewport limit,
  keep the heading visible, and scroll longer member lists instead of clipping
  the final agent's token usage.
- Move runtime search settings onto shared Field labels and errors, isolate
  control and advanced-panel IDs per instance, and keep secret actions separate
  from labels. Labeled segmented controls now expose one named group; runtime
  and skill-source choices retain their existing draft and save behavior.
- Replace decorative microtext in configuration groups and run details with
  existing readable typography roles, improve execution-history text hierarchy,
  and simplify RichMail connection guidance to one heading.
- Unify sidebar search actions with the shared IconButton, retain independent
  search/clear/create commands, and use readable shared text with concise bilingual
  placeholders and full accessible names within the existing sidebar width.
- Reuse the shared segmented control for skill-source authentication, preserve
  selected-option keyboard focus, suppress disabled hover styling, and align
  icons with their labels without changing credential or save behavior.
- Associate shared field descriptions and errors with their exact input or select,
  preserve current caller validation attributes after native errors recover, and
  complete missing labels in channel configuration and pairing creation forms.
- Refine shared status and supporting-text colors, pair solid danger/success
  controls with theme-aware foregrounds, and replace invalid gradient inputs in
  control color mixing. Remove unused info-color tokens and duplicate Button/Badge
  recipes; add CSS color-type checks and a rendered semantic-color review fixture.
- Align compact buttons and selects with matching input sizes, improve field and
  settings label/description hierarchy, and remove redundant description spacing.
  Settings selects retain shared keyboard focus styling; capability directory rows
  now use the shared outlined surface without private border or hover overrides.
- Improved placeholder readability in shared text, multiline and search fields
  across themes and strengthened Rain's shared supporting-text contrast while
  preserving existing typography, size and input behavior.
- Preserve pinned sessions when another page saves its conversation tabs, and
  synchronize same-owner navigation changes without restoring another account's data.
- Keep the shared sidebar rail compact in both languages, using clear short
  navigation labels with the same icon geometry and behavior.
- Unify catalog filter controls, keep one context-usage detail without duplicate tooltips or Escape focus jumps, and remove obsolete frontend types and helpers while retaining current behavior coverage.
- Share one User/Assistant reading layout, keep queue and file-card styles out of data models, and localize file actions without changing message or workspace scope.
- Reuse shared panels, typography and activity rows in private conversations and WorkGraph inspectors; preserve native message-edit geometry, guard IME shortcuts, and move Composer spacing and textarea effects out of state models.
- Share the Login and Setup access layout, brand heading scale, and filled Panel surfaces; retain credential and initialization behavior with isolated browser coverage.
- Consolidate WorkGraph node and relation inspectors into one shared surface pattern, reuse semantic status badges, and verify exact selection, workspace-file actions, and zoom behavior in real browsers.
- Centralized control and surface radius tokens, restored Tour highlighting,
  and replaced undefined UI color references with existing semantic tokens.
  Added gates for missing static variables and broken theme aliases, plus
  browser checks for token resolution and non-blocking Tour target actions.
- Reused shared buttons for Composer attachment previews and removal, preserving
  independent actions, Session draft boundaries, and local preview cleanup.
  Removed duplicate chip and input-shell radius overrides so their shared
  recipes remain the visual owners.
- Consolidated Agent authorization rows and Skill cards onto shared ListRow,
  Catalog, and typography primitives while preserving switch-only commands,
  disconnected-access removal, and locked or pending Skill behavior. Catalog
  creation actions now reuse shared Button focus and disabled states.
- Unified sidebar row and skeleton density, list surfaces and muted states under
  shared ListRow props. Static rows no longer advertise hover interaction, and
  busy Feishu connection choices retain accessible disabled semantics. Native
  control ownership checks now recognize aliased React element factories.
- Unified technical-text and verification-code field presentation through shared
  form roles, removed private input and search typography, and extended the
  visual ownership gate to native form and selection controls. Native values,
  leading zeros, validation attributes and form submission remain unchanged.
- Consolidated catalog-card hit areas and list actions under shared owners,
  restored keyboard visibility for row actions, and removed consumer color,
  hover and shadow overrides with an import-aware contract gate. Select sizes
  now cover compact filters and large form fields without local height recipes.
- Removed unused frontend compatibility APIs, presentation helpers and duplicate
  runtime state; provider setup notices now reuse the shared inline component.
- Aligned default shared Button, Form and Select typography and geometry with
  the design contract; unavailable primary actions are neutral, explicitly busy
  actions retain their tone, and disabled buttons no longer react to hover.
- Added representative WebKit coverage and measured control geometry to the UI
  browser gate, using each host's native keyboard traversal behavior.
- Connected the mobile conversation switcher to shared modal focus, scroll
  locking, Escape dismissal and focus restoration while retaining its history
  filtering and compact sheet geometry.
- Moved canonical navigation paths into one shared contract, removing upward
  Feature-to-App imports. Removed all known upward dependency exemptions after
  separating page action slots, auth identity, directory events, Markdown
  capabilities, and conversation-tab commands from shared presentation.
- Moved generic UI hooks and clipboard access into shared React/browser owners,
  and released copy-feedback timers and late feedback when consumers unmount.
- Unified the login form with shared fields, panels, typography, and buttons,
  preserving keyboard submission and unknown-result recovery behavior.
- Consolidated single- and multi-select triggers, dialog close actions, and tab
  dismiss buttons under shared component owners while retaining their existing
  geometry and behavior. Deferred typography, spacing, and palette tuning to a
  separate visual phase.
- Fixed nested overlay dismissal and focus restoration, including overlays
  initially mounted inside dialogs; disabled selectors now close immediately
  and remain closed when re-enabled.
- Fixed action-menu keyboard focus under reduced motion and prevented hidden
  mention pickers from consuming another component's navigation keys.
- Separated conversation-tab selection from pin and close hit areas so narrow
  tabs remain directly selectable without triggering a neighboring action.
- Unified primary navigation and pinned conversations under one sidebar rail
  action owner, including consistent icon geometry, caption typography,
  current/focus states, and counter-safe accessible names.
- Unified Agent, Personal, and Room avatar pickers under one trigger and image
  Choice contract, removing private selection shadows and repeated focus,
  disabled, label, and arrow styling.
- Separated shared feedback lifecycle policy from icon and color rendering, so
  data models no longer carry component constructors or visual class names.
- Split Select Menu state and anchored geometry from its shared visual recipe,
  removing style exports and an unused surface argument from the menu model.
- Centralized Room history, Composer, message-reference, Slash, and scheduled
  picker geometry behind shared semantic overlay presets while preserving the
  current placement and dimensions.
- Unified authentication, Agent settings, Channel authorization/account, and
  scheduled rebind recovery surfaces behind the shared inline notice contract.
- Replaced Composer model-owned color classes and pulsing status copy with
  semantic tones and shared LoadingOrb variants, including deterministic CSS
  frame cycles and a static reduced-motion state.
- Replaced private shimmering message-status copy and vertically scrolling
  Unicode spinners with one fixed-footprint shared LoadingOrb, preventing
  thinking, tool, and reply activity from appearing to shake the conversation.
- Moved Launcher recent-entry colors, shadows, sizing, shape, and animation out
  of its business model, and routed both entry and handoff actions through the
  shared Button contract.
- Restored Launcher recent entries to their original transparent treatment,
  replacing repeated robot glyphs with stable semantic-color identity dots.
- Reduced the collapsed-sidebar Surface Header reservation to the restore
  control's real footprint while retaining macOS window-control safety.
- Matched the Composer WorkGraph dock's Agent and graph actions to the shared
  32px icon hit target, with an 18px graph glyph for balanced proportions.
- Added a shared 40px activity-toolbar frame around grouped 32px actions so
  WorkGraph Agent docks keep consistent breathing room instead of hugging edges.
- Replaced full-width private WorkGraph mutation error cards with shared compact
  notices capped at 384px, removed repeated operation identities, and preserved
  full-width code, JSON, image, and log output.
- Optically rebalanced the empty Room introduction while preserving its true
  center line and the left-aligned scanning pattern inside suggestion blocks.
- Split Agent private-thread data projection from its density and typography
  recipes so directory sizing can change without modifying the business model.
- Replaced the final Goal model-owned color classes with semantic Badge tones
  shared by its lifecycle label and leading status icon.
- Moved Goal lane geometry out of its pure state model and placed the floating
  status strip on the shared transparent, shadow-free Panel surface.
- Moved Goal lifecycle and WorkGraph-binding labels onto the shared Badge
  contract, and rendered usage as plain semantic metadata instead of a private pill.
- Replaced Mermaid-specific mode and copy controls with shared primitives, and
  moved rendered-diagram activation from a simulated role to one native button.
- Restored the four empty-conversation suggestions as consistent transparent,
  border-only action blocks without card shadows or persistent fill.
- Centered the shared empty-conversation welcome surface within the visible DM
  and Room viewport, tightened its suggestion layout, and corrected spacing
  around interpolated Agent names in Chinese headings.
- Unified removable Agent tags and selected Room Skills under one accessible
  chip primitive, separating menu and removal hit targets and honoring disabled
  state for both actions.
- Removed the fake clickable wrapper around protected default-model switches;
  the shared switch now owns native disabled semantics and routes protected
  activation through one accessible hit target, while tolerating host surfaces
  that do not expose pointer-capture APIs.
- Moved Room name editing and tool-permission scope radios onto the shared Input
  and native RadioChoice contracts, including consistent focus, disabled, and
  selection semantics.
- Extended the frontend governance plan with a final reverse-audit phase that removes missed private controls, duplicate rules, dead adapters, unused exports, stale state branches, and outdated documentation before the refactor is considered complete.
- Moved Loop and saved WorkGraph picker rows onto the shared list and listbox interaction owners, including consistent keyboard, focus, selection, and disabled behavior.
- Routed empty-conversation suggestions, user-message expansion, subagent task links, Artifact external actions, and Composer context usage through the shared Button contracts instead of private control styling.
- Unified Thought, process-summary, and tool-run disclosure rows under one message-domain toggle with shared arrow, focus, typography, and semantic status behavior.
- Reused shared icon-action and list-row owners for pinned-conversation removal and Room fallback history navigation while preserving the drag target as an explicit geometry-owned control.
- Added one shared Unicode-normalized client search contract so capability, conversation, contact, and memory directories can declare searchable fields without reimplementing trimming, case folding, empty-query behavior, or substring matching.
- Unified ordinary expandable App sections under the shared `UiDisclosure`
  contract, so Channel instructions, pairing details, Connector scopes, Skill
  import help, Scheduled run history/settings, and Execution run facts now
  share native semantics, focus, arrows, density, typography, and boundaries.
- Moved subagent task entries onto the shared dense List and Typography
  contracts, with running-avatar emphasis now owned by the shared avatar state.
- Moved Room history entries onto the shared whole-row List contract, unifying
  hover, selection, focus, and keyboard behavior while isolating inline actions;
  timestamps now align at the title edge, while IM sessions live below a shared
  labeled divider and use a quiet identity line instead of a competing pill.
- Replaced the spreadsheet preview's private filled sheet pills with the shared
  underline Tabs contract while preserving scrolling, truncation, and selection.
- Moved the conversation round navigator preview onto shared semantic
  typography while documenting its ruler and whole-card buttons as tested
  geometry-owned hit-target exceptions.
- Unified Action Menu, Workspace context-menu, and Room model rows on one native
  shared menu-item button, including disabled, active, danger, hover, and focus
  states.
- Defined the frontend design-system migration as two ordered phases: first
  consolidate private implementations under shared owners, then validate and
  refine the unified system across Web, macOS, and Windows for density,
  typography, hit targets, interaction states, flicker, and layout consistency.
- Standardized structured question decisions and text hierarchy on shared Button
  and Typography owners while preserving native question input semantics.
- Standardized every content, directory, and filter tab on the neutral underline
  treatment, including Pairing status filters; finite MCP configuration choices
  now use the form SegmentedControl instead of masquerading as navigation tabs.
- Standardized Composer permission decisions on shared Button, split-action,
  Form, Menu, and Typography owners without changing approval payloads.
- Centralized Select, Slash-command, and Room Skill listbox option DOM on one
  shared menu row primitive, including selection ARIA and active-state data.
- Unified Assistant footer copy, branch, and memory actions plus ToolBlock
  permission and result actions on the shared micro Button/IconButton states;
  referenced-memory popovers now use the App typography contract.
- Unified WorkGraph search, zoom, fit, locate, expand, inspector-close, and
  save-as-sketch actions on shared Button and IconButton primitives, while
  retaining graph nodes and edge hit targets as geometry-owned interactions.
- Unified Workspace file Header actions on the shared compact IconButton and
  moved presentation thumbnails, paging controls, labels, and localized action
  names onto shared Choice, IconButton, and Typography contracts.
- Removed the misleading Workspace-only toolbar action adapter: Capability and
  Operations page headers now use the base micro text Button contract directly,
  icon-only Header actions use IconButton, and Operations page modes use the
  same neutral line treatment as other page-level sections.
- Replaced the stretched Skill and Connector directory mode capsules with one
  shared single-line, neutral-underline page-mode contract; long labels no
  longer wrap or divide the full content width in narrow windows.
- Removed the redundant message-specific action-button adapter: message rerun,
  edit, copy, and stop controls now use shared micro Buttons, with copy
  confirmation represented by the new shared success tone.
- Added shared 20px icon and 24px text micro-action sizes, then adopted them
  across Composer directory, queue, attachment, Goal, and Room execution
  controls so dense toolbars no longer redefine button states per page.
- Unified Composer action, Session permission/model, Room model, and nested back
  triggers on shared Button primitives while retaining specialized Agent rows
  and context-usage visualization behavior.
- Moved Room Agent, member-stack, and mobile conversation Header triggers onto
  the shared Button state contract while preserving their identity layouts.
- Unified Room history and Session-edge controls on shared Button, Form, List,
  and native mixed-checkbox contracts without changing conversation behavior.
- Unified Sidebar collapse, guide, update, and logout actions on the shared
  round IconButton contract while retaining Settings as a real route link.
- Standardized Agent private-thread navigation on shared selectable ListRow
  behavior and semantic typography without changing thread ownership or data.
- Moved Agent Options section navigation onto the shared Button active-state
  contract while preserving its desktop rail and narrow-window tab layout.
- Kept centered onboarding Tour cards inside the active window after live
  desktop or browser resizing, including narrow macOS and Windows layouts.
- Standardized onboarding Provider selection on shared ListRow, Badge, and
  Button primitives, removing its private radio marker and final raw buttons
  without changing the staged save, test, or default-selection workflow.
- Added a shared 40px dense ListRow contract and adopted it for Memory catalog,
  Memory index, and Room member selection; nested Room participation choices
  remain independently operable without triggering the parent row.
- Moved Agent permission-mode cards onto the shared neutral Choice contract so
  selected borders, backgrounds, focus treatment, and shadow policy no longer
  live in the Agent settings page.
- Centralized icon-only segmented options in the shared form control, then moved
  Contact directory view modes and external Skill source filters onto the
  shared segmented and choice-selection contracts.
- Standardized WorkGraph editor loading, title, command, version label, and
  version-selection chrome on shared Typography and Choice primitives while
  preserving per-version disabled state and horizontal browsing.
- Removed the Dialog action class adapter: shared decision actions, Goal editing,
  and WorkGraph save/edit flows now render `UiButton` directly, leaving button
  emphasis, shadows, sizes, and interaction states with one DOM owner.
- Unified WorkGraph artifact cards and narrow comparison panes on shared Panel,
  Button, Badge, Tabs, typography, and loading contracts, removing their private
  selected-tab shadow and text recipes.
- Moved the streamed generative-UI placeholder, title, and container geometry
  onto shared Skeleton, typography, and semantic shape contracts.
- Centralized Home sidebar loading rows on the shared Skeleton tone, shape,
  animation, and reduced-motion contract so chat and contact placeholders no
  longer maintain private visual recipes.
- Unified Skill import, external search, preview, and source-management chrome
  on shared segmented controls, states, panels, actions, typography, and spinners.
- Standardized Channel and Connector authorization dialogs on shared form,
  panel, status, typography, badge, and loading contracts.
- Unified every Capability sidebar row and its filtered empty state on shared
  list, typography, selection, and semantic shape contracts.
- Standardized the Skill directory on shared card actions, semantic typography,
  resource states, page tabs, icon buttons, and loading indicators; Skill and
  Connector directory modes now share the same compact navigation treatment.
- Unified Channel account, QR login, verification, progress, and destructive
  waiting surfaces on shared panel, typography, shape, and spinner contracts.
- Standardized populated Pairing groups, rows, metadata, and expandable
  technical identities on shared panel and typography contracts.
- Moved Pairing empty/search states and the Skill update summary onto shared
  resource-state, panel, list-row, typography, and loading recipes.
- Standardized the object identity block beneath capability detail navigation:
  Skill, Connector, custom MCP, Loop, and WorkGraph now share leading identity,
  title, metadata, description, and responsive action alignment.
- Unified Skill, Connector, custom MCP, Loop, and WorkGraph detail pages on one
  capability-owned content axis and “directory / current item” header; opening
  a WorkGraph detail no longer leaves the directory title or search controls
  above the selected object.
- Removed the remaining shared-UI spinner forks: Mermaid module and render
  states now use one accessible recipe, while Session creation replaces the
  plus action with a real busy indicator instead of rotating the action glyph.
- Centralized route, Workspace, resource-state, and decision-action loading
  indicators under one semantic size, color, and reduced-motion recipe while
  retaining the local animated Nexus cat for brand-level startup waits.
- Moved shared sidebar empty and recovery guidance onto semantic caption roles,
  the common Surface shape, and the standard compact Button so Chat and Contact
  sidebars no longer inherit a private text and action recipe.
- Centralized Onboarding Tour titles, descriptions, items, progress, and target
  highlighting under the shared typography and surface recipes, with a retained
  button behavior contract for future guide changes.
- Unified the Composer Task, Room collaboration, and WorkGraph activity chips
  on one semantic typography recipe, and moved their compact icon actions onto
  the shared button primitive so the three status surfaces cannot drift apart.
- Unified narrow-window app and Room headers behind one platform-aware layout:
  macOS follows the measured native window-control center, while Windows and
  browsers retain the 52px client-area height and the same shared back action.
- Extended that platform-aware geometry through Room conversation switchers and
  auxiliary overlays, and centralized their circular icon actions, compact rows,
  semantic typography, and overlay layers in shared UI primitives.
- Unified Room Thread and subagent full-screen surfaces on the same semantic
  dialog layer and platform-aware mobile Header, including a stable flex layout
  for long subagent task directories.
- Centralized Workspace Surface title, subtitle, compact navigation, and toolbar
  action typography on the shared role map, and replaced its remaining private
  identity radius with the shared control shape.
- Moved default ListRow title/description text onto semantic typography roles
  and added a first-class pill shape to shared badges so feature pages no longer
  override badge radii directly; Connector and Custom MCP two-line directories
  now consume those shared content slots instead of rebuilding row typography.
- Reorganized Skill details around a shared responsive capability layout: long
  instructions keep a readable main column while badges and per-Agent controls
  occupy a bounded configuration rail that moves before content on narrow windows.
- Removed unimplemented placeholder Connectors from the server catalog and
  reorganized the available directory into only its real capability groups,
  with category filters derived from the products that are actually present.
- Unified Channel loading, typography, actions, and platform identities with
  Capability shared primitives; DingTalk, WeCom, WeChat, Feishu, Telegram, and
  Discord now use distinct monochrome brand silhouettes instead of colored or
  repeated placeholder icons.
- Standardized scheduled-task board suggestions, column labels, loading motion,
  typography, button focus, and surface radii through shared UI recipes.
- Moved scheduled-task cards and attention details onto shared catalog cards,
  panels, buttons, badges, typography roles, and reduced-motion loading recipes.
- Unified scheduled-task run rows, diagnostics, output errors, retry actions,
  and artifact actions with shared typography, panel, radius, and button owners.
- Unified the post-navigation content offset across Skill, Connector, Custom MCP,
  Loop, and WorkGraph details so every secondary capability page shares one rhythm.
- Kept shared segmented-control labels on one line so Skill source tabs and other
  compact selectors no longer grow unevenly when a toolbar becomes narrow.
- Standardized scheduled-task form grouping, rebind guidance, helper copy, and
  advanced disclosure chrome through shared panels, typography, and radius roles.
- Replaced Scheduled date/time plus triggers and text month navigation with shared
  accessible buttons, semantic picker choices, and labeled anchored overlays.
- Consolidated the Contacts directory, create entries, Agent cards, metadata, tags,
  empty results, and view switcher onto shared catalog and design-system owners.
- Extracted Agent auto-save status into a tested Contacts header component backed
  by shared loading, icon-action, typography, surface, and overlay-layer recipes.
- Split Contacts communication directory, status projection, and pure naming/filter
  model from chat orchestration; its friend rows, candidate picker, empty/loading
  states, form actions, and spinner now use shared design-system owners.
- Fixed the shared Workspace header's container breakpoint so narrow detail pages
  show one compact tab menu instead of overlapping it with the full tab strip.
- Gave Loop and WorkGraph details the same stable resource avatar used by their
  directory entries, and moved seeded-avatar rounding onto semantic shape roles.
- Unified the 559px App shell handoff contract so capability detail navigation
  remains visible in medium desktop windows and yields only to the mobile header.
- Unified Conversation, Provider, and read-resource reliability strips on one tested inline
  feedback owner for radius, tone, typography, recovery actions, and pending state.
- Moved Launcher, chat-sidebar, and contacts-sidebar directory refresh failures
  onto the same inline feedback owner while preserving their safe read-only retry.
- Reused the inline feedback and spinner owners for Custom MCP form notices,
  Loop launch failures, and Room Skill resource errors.
- Consolidated Room Skill form failures, subagent refresh recovery, Agent full-access
  warnings, reply-limit warnings, and stale Memory notices onto the same inline owner.
- Standardized Workspace file-preview loading indicators on shared compact and
  32px canvas Spinner roles with one reduced-motion contract.
- Moved Memory directory, document, runtime-write, save, refresh, and delete
  loading states onto the same semantic Spinner scale.
- Standardized General, Personal, and Browser settings loading states on shared
  page and compact action Spinner roles with reduced-motion behavior.
- Unified Provider directory, test, sync, model mutation, and dialog loading
  states through the same semantic Spinner scale.
- Moved Operations member, project, subscription account, and plan commands to
  the shared compact Spinner role.
- Standardized Connector card, scheduled-directory refresh, and Channel pairing
  dialog loading indicators on shared semantic Spinner roles.
- Unified Goal edit submission and lifecycle refresh indicators on the shared
  medium action Spinner role.
- Standardized WorkGraph canvas, action, availability, and revision loading
  states on the shared semantic Spinner scale.
- Unified Subagent transcript loading and task command indicators through the
  shared medium and compact Spinner roles.
- Standardized Room history deletion, Thread waiting, and collaboration
  activity indicators on the same semantic Spinner scale.
- Unified message question submission, Subagent task status, Assistant fork,
  image-detail, and WorkGraph-source loading through shared Spinner roles.
- Standardized Composer Connector loading and Room Agent model updates on the
  shared medium and compact muted Spinner roles.
- Unified Agent Skill directories, Skill mutations, and private-domain thread
  loading and refresh indicators through shared muted Spinner roles.
- Standardized Launcher submission, desktop update, and Provider onboarding
  loading indicators through the shared Spinner motion and tone contract.
- Unified CC Switch Provider import and legacy Operations route loading through
  shared Spinner roles, removing the last non-Workspace production border spinner.
- Standardized Workspace directory, upload, and desktop application discovery
  loading through shared Spinner roles.
- Extended the Spinner ownership gate from shared primitives to every production
  frontend source file while excluding tests and the development gallery.
- Replaced the Workspace header's physical Agent directory identifier with the
  Agent display name and a user-facing relative path, then unified its trail
  with Skill, Connector, Loop, and WorkGraph detail headers on one shared
  breadcrumb component.
- Established a single frontend engineering and design-system contract, and
  centralized high overlay layers and responsive dialog geometry behind semantic
  APIs to prevent page-specific stacking and small-window sizing drift.
- Added jsdom-backed behavior tests for core UI primitives and unified ordinary
  checkbox, search clear, segmented selection, and view-filter group semantics.
- Replaced numeric select/action menu layers with named design tokens, added
  consistent menu keyboard traversal and focus return, and hardened shared
  dialogs with nested Escape ordering, focus-loop, backdrop, and scroll-lock
  behavior contracts.
- Unified rich anchored overlays such as icon, memory, and Room model pickers
  on the shared semantic popover layer instead of feature-owned z-index values.
- Unified large WorkGraph compare and metadata dialogs on one responsive
  workbench size/viewport contract, including the compact-window inset.
- Removed the login primary action's page-owned shadow and consolidated brand
  artwork projection into Login-owned visual recipes; also removed decorative
  avatar and WorkGraph card shadows plus a redundant Launcher tooltip shadow.
- Replaced the Launcher-specific recent-entry tooltip with the shared accessible
  tooltip behavior, and moved WorkGraph distillation onto the common responsive
  workbench dialog geometry.
- Reduced the product-source arbitrary-shadow debt baseline to zero by mapping
  previews and graph nodes to semantic elevation, and drag/highlight states to
  structural borders or rings.
- Reduced the product-source numeric z-index debt baseline to zero, corrected
  Session Navigator previews to the shared popover material/layer, removed the
  duplicate `UiPanel` inset variant, and added enforced behavior suites for
  shared panel, list, badge, counter, and resource-state primitives.
- Expanded the development-only UI contract gallery into an exhaustive catalog
  of real `shared/ui` components across foundation, content, interaction, and
  Workspace surfaces. Its Chinese and English fixtures now switch with the
  locale, section/theme/locale remain reproducible in the URL, and a contract
  test fails when a newly exported shared React component is not inventoried;
  the gallery remains outside production build entries.
- Consolidated provider setup, Loop and WorkGraph pickers onto one compact
  dialog viewport, moved natural-height contact, guide, Skill-source and
  Connector directories onto one compact maximum, and moved provider import
  and Room management dialogs onto shared adaptive height and compact-window
  inset contracts.
- Consolidated remaining Channel, Connector, Skill, scheduled-task, Mermaid,
  and expanded WorkGraph dialog heights onto semantic compact, adaptive, and
  workbench viewport contracts; refined shared single-line prompts with a
  narrower decision width, standard header/input primitives, and a clear solid
  primary action for Workspace create and rename flows.
- Moved Composer image and text attachment previews onto named visual/document
  viewer geometry, and removed the remaining feature-owned Dialog Shell width
  overrides from Provider settings and scheduled-task editing.
- Routed ordinary product actions and external navigation through the shared
  Button primitives, preventing feature code from copying internal button
  recipes while preserving each action's size, tone, and loading state.
- Added a native Select form primitive and moved project, subscription,
  password, and deployment-member fields onto shared form-control ownership,
  with an architecture gate for internal style imports and unowned selects.
- Added a shared semantic typography system for App chrome, aligning the
  documented type scale with theme tokens and centralizing font family, size,
  line height, weight, tracking, and text tone behind typed roles. The UI
  contract gallery now shows the full hierarchy, and architecture checks reject
  arbitrary pixel aliases for the standard scale.
- Migrated Settings titles, descriptions, labels, and segmented options onto
  semantic typography roles; removed its duplicate segmented control so shape,
  density, selected-state contrast, and no-shadow behavior have one owner.
- Routed all Settings single-line, multiline, and compact checkbox fields
  through shared Form primitives, removing feature-owned input geometry and
  enforcing that ownership with an architecture contract.
- Unified the main Settings and Provider directories on one navigation Pattern;
  their typography, control geometry, current-page state, hover treatment, and
  Button DOM now follow shared owners instead of page-local class recipes.
- Migrated the complete Personal settings surface to semantic Typography,
  Badge, Shape, and Settings Card owners, including identity metadata, token
  usage metrics, avatar state, password labels, and validation feedback.
- Migrated Provider settings titles, labels, descriptions, model identifiers,
  status badges, counts, and fallback icons to shared semantic Typography,
  Badge, and Shape owners, with a gate against page-local font recipes.
- Migrated Browser settings connection, install, recovery, and CDP surfaces to
  shared semantic Typography, Badge, Resource State, and Settings Card owners,
  including a contract that rejects page-local font and radius recipes.
- Collapsed the standalone Settings text panel into its existing icon rail on
  narrow desktop windows so every Settings section keeps a usable content plane.
- Consolidated Operations member, subscription, and project surfaces onto shared
  semantic Typography, Badge, Resource State, Settings Card, and Control Label
  owners, removing the redundant Subscription-only loading and empty wrappers.
- Completed the Settings-wide semantic typography migration for permission,
  workspace-path, and runtime validation copy, with a domain gate preventing
  future page-local font and arbitrary-radius recipes.
- Standardized Connector, Custom MCP, and Skill detail chrome on shared Button,
  LinkButton, Typography, Badge, and Resource State owners, removing duplicated
  breadcrumb action styling and the Skill-only failure card; the shared MCP tool
  header now moves retry actions onto a full-width-safe row in narrow windows.
- Standardized Loop directory and detail chrome on shared semantic Typography,
  Badge, Panel, Resource State, and Button owners; narrow section actions now
  move to their own row instead of compressing or wrapping vertically.
- Unified all capability directory titles, descriptions, section headings,
  counts, identity frames, and desktop/mobile action placement behind the
  shared Capability page layout and Workspace content Header contracts.
- Split WorkGraph capability detail rendering from its resource directory and
  moved directory metadata, detail actions, target summary, and canvas shell to
  shared semantic Typography, Button, Panel, and Surface contracts.
- Moved floating feedback and shared resource states onto named surface, layer,
  typography, shape, and Button contracts, with DOM tests for recovery,
  dismissal, live-region behavior, and auto-dismiss timing resets.

### Removed

- Removed the redundant uppercase overline typography role and recipe after
  migrating its production consumers to their existing semantic text roles.
- Removed the Gallery-only catalog icon-action adapter; production catalog text
  actions continue to use the shared Button.
- Removed the unused metadata-grid and glass-magnifier components, their Gallery
  demos, and three magnifier-only images after checking all production entries
  and repository references. The production glass switch remains in use.

### Fixed

- Fixed hidden Goal continuations attaching their report to an earlier question
  after history reload, hiding that question's answer and duplicating the report.
  Empty runtime inputs now preserve their Goal round, and cached history is rebuilt.

- Unified WorkGraph command editing and saving around the current Draft and its active command; stale selections are rejected, lost responses can be checked without resubmitting, and historical commands from the same source retain independent drafts. Deleting a command preserves its editable draft.

- Keep saved WorkGraph forms editable in the same dialog. Renaming or editing
  metadata restores the save action, and each confirmed save refreshes the Draft
  revision so subsequent saves update the same command without a model round.

- Prevent WorkGraph editor version switches and delayed refreshes from applying
  an older sketch; failed reads block changes until recovery, and rejected
  applications offer a refresh action. Return the committed editor revision
  directly so concurrent status reads cannot race with its mutation receipt.

- Confirmed WorkGraph sketches and command-name edits now save directly in one
  database transaction, without a background model round; the UI confirms the
  persisted command immediately. Existing pending saves can be completed this way.
- WorkGraph save-dialog renames now survive reopening the existing sketch editor.
  Save receipts verify persisted content before reporting success, and confirming
  a save repairs stale revision markers without creating a duplicate command.
- Name standard dialogs automatically from their visible headings, including
  Skill import, Connector authorization and Composer pickers. Keep nested and
  same-named instances isolated, update names across steps, and preserve explicit
  preview names without duplicating title wiring in each page.
- Preserve other open Session tabs when closing the last visible tab creates a
  replacement conversation, including tabs opened from another page while the
  replacement is pending. Remove the remaining whole-tab-set overwrite command.
- Preserve open Session tabs when an older conversation list refreshes. Creating
  a Session or selecting history appends its tab; closing removes only that tab
  while retaining other open tabs and pinned Sessions across reloads.
- Restored conversation WorkGraph draft cards when `nexus.command` returns its
  structured result through the MCP wrapper.
- Accepted bridge-preserved JSON integer tokens at the command schema boundary,
  restoring WorkGraph draft revisions with numeric revision and position fields.
- Fixed shared Dialog layer variants being overridden by a legacy `z-index: 50`
  fallback; ordinary, nested, interaction and system dialogs now resolve through
  their semantic overlay tokens in the browser.

## [0.1.40] - 2026-09-04

### Added

- Added Control-backed server Web authentication, first-run setup, member and
  subscription administration, signed principals, and migration of existing
  accounts without moving Agent or workspace data.
- Added complete custom MCP server management and the RichMail Connector,
  including per-owner availability, live tool discovery, encrypted credential
  rotation, and isolated recovery for unreadable legacy records.
- Added an 8-10-player Avalon Room Skill with a permanent Agent moderator,
  private roles, voting, missions, rejections, and assassination.
- Added an administrator-selected subscription default model and FailureCore v1
  machine-readable failure classification.

### Changed

- Moved identity and subscription authority to `nexus-control`, the public
  landing page to `nexus-atlas`, and source deployment to a coordinated
  `control + nexus + nginx` stack. The Product root now enters the authenticated
  Launcher.
- Improved dense WorkGraph routing, subgraph avoidance, path focus, history
  selection, endpoint controls, and the near-fullscreen detail view.
- Simplified failure and recovery guidance across conversations, Goals,
  settings, Connectors, Channels, and desktop flows while keeping diagnostics
  out of user-facing copy.
- Refined compact Agent, Skill, menu, Composer, Room, and sidebar layouts,
  including a desktop sidebar that collapses into the shared window chrome.
- Updated Feishu, DingTalk, and personal Weixin clients with current connection
  lifecycle behavior.
- Updated the bundled nxs runtime to v0.1.31 and the runtime bridge to v0.1.32,
  adding per-turn AutoMemory control and clearer terminal Provider failures.

### Fixed

- Kept GLM 5.3 WorkGraph sketch generation in its required thinking mode while
  using the lowest supported reasoning effort and allowing enough output tokens
  and time to finish.
- Stopped development and ad-hoc macOS app launches from probing the legacy
  Keychain item, avoiding a password prompt after every local rebuild.
- Prevented duplicate or unsafe retries after uncertain outcomes across
  scheduled tasks, Agents, Skills, workspaces, Memory, Goals, Rooms, Providers,
  subscriptions, and password changes.
- Preserved authoritative Provider retry progress and terminal errors, and
  prevented runtime error details from ending a round before its final result.
- Reconciled migration-number collisions and kept Control data out of legacy
  Nexus state migration.
- Stabilized Room execution snapshots, activity geometry, private collaboration,
  transcript grouping, and user-only AutoMemory extraction.
- Hardened Connector credential migration, custom MCP route identities, legacy
  record recovery, and Feishu, Weixin, and WeCom connection lifecycles.
- Bounded large workspace previews and fixed desktop state migration, Windows
  browser-extension setup, anchored overlays, font fallbacks, and narrow-window
  sidebar geometry.

## [0.1.39] - 2026-08-31

### Added

- Added localized WorkGraph templates for deep research, build-and-ship delivery, decision briefs, and review-and-improve workflows, with adaptive branches, verification gates, and reusable Slash commands.
- Added Agent directory grid/list views, business and vibe tags, and business-tag, Provider, and permission filters.
- Added default-on controls for automatic long-term memory extraction and background memory consolidation.
- Expanded the built-in model catalog with current context, output, reasoning, and image capabilities across major providers, including GLM-5.3.

### Changed

- Updated the bundled nxs runtime to v0.1.30 with stricter automatic memory retention, broader model capability data, and more reliable background maintenance.
- Reduced ordinary DM and Room prompt churn while preserving Execution, WorkGraph, Runtime Graph, collaboration, and reply-routing context.
- Reworked tools, thoughts, memory events, and Room Thread activity into compact timelines with stable live grouping, bounded details, and final answers outside the process rail.
- Unified Nexus runtime commands, communication, and authorization behind round-scoped tools while keeping third-party and Connector tools independently selected and approved.
- Replaced generated first-conversation welcomes with static DM and Room empty states that do not enter history, unread counts, or model context.
- Filled missing model metadata from the built-in catalog and stable defaults when Providers omit limits or capabilities.

### Fixed

- Stabilized Room execution activity, public replies, task identity, WorkGraph recovery, self-assignment review, and long Slash-template input.
- Preserved shared transcripts and hidden runtime context across NXS/Claude switches, while starting a fresh compatible session when a runtime cannot resume the old one.
- Forked active Room sessions after Connector changes so newly selected tools are available on the next round.
- Restored custom Provider identifiers, non-default model deletion, default-model guard messages, and latest DM reply previews.
- Let Agents discover and message their paired external DMs across chat and scheduled delivery with exact owner, Session, and pairing checks.
- Kept the browser extension folder visible after setup, improved macOS DMG detach recovery, and waited for Windows WebView2 to release files before moving the desktop data directory.
- Updated frontend production dependencies to resolve reported security issues.

## [0.1.38] - 2026-08-24

### Added

- Added bundled Word, PDF, PowerPoint, and Excel Skills with task-specific reading, creation, editing, conversion, and delivery workflows.
- Added persistent pinned conversation shortcuts with tab pinning, drag reordering, and direct unpin actions.
- Expanded the Guide Center and added a source-checked Nexus product guide Skill covering conversations, Rooms, Goals, WorkGraphs, proactive follow-up, scheduled tasks, Browser, external services, messaging, and settings.

### Changed

- Updated the bundled nxs runtime to v0.1.29 with authoritative native runtime-state reporting and a leaner Nexus-focused tool surface.
- Improved WorkGraph saving with the full interactive preview, inline Slash-name validation, missing-context questions, and an explicit review checkpoint.
- Updated welcome messages to guide users toward Nexus features while preserving Room `@AgentName` routing rules.

### Fixed

- Switched Windows WebView2 minimize and tray handling to its supported visibility lifecycle, preserving desktop and keyboard input after updates and window restores.
- Prevented keyboard actions on nested list controls from also opening their parent row.

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
