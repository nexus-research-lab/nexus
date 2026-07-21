import { execFileSync } from "node:child_process";
import { copyFileSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { tmpdir } from "node:os";

import { verify_completed_round_replay_uses_event_slice } from "./operation-stage-replay-verifier.mjs";
import { verify_html_artifact_opens_browser_srcdoc } from "./operation-stage-browser-verifier.mjs";

const script_dir = dirname(fileURLToPath(import.meta.url));
const web_root = dirname(script_dir);
const out_dir = join(tmpdir(), "nexus-operation-stage-projector");
const operation_dir = join(out_dir, "src/features/conversation/operation");
const shared_editor_dir = join(out_dir, "src/features/conversation/shared/editor");

rmSync(out_dir, { recursive: true, force: true });

execFileSync(
  join(web_root, "node_modules", ".bin", process.platform === "win32" ? "tsc.cmd" : "tsc"),
  [
    "--project",
    "tsconfig.json",
    "--outDir",
    out_dir,
    "--noEmit",
    "false",
    "--declaration",
    "false",
    "--sourceMap",
    "false",
  ],
  {
    cwd: web_root,
    stdio: "inherit",
  },
);

writeFileSync(join(out_dir, "package.json"), "{\"type\":\"module\"}\n");

// The app uses bundler-style extensionless imports. Node's ESM loader needs
// matching files when executing the compiled projector directly.
copyFileSync(join(operation_dir, "operation-tool-catalog.js"), join(operation_dir, "operation-tool-catalog"));
copyFileSync(join(operation_dir, "operation-tool-inference.js"), join(operation_dir, "operation-tool-inference"));
copyFileSync(join(operation_dir, "operation-desktop-intents.js"), join(operation_dir, "operation-desktop-intents"));
copyFileSync(join(operation_dir, "operation-file-documents.js"), join(operation_dir, "operation-file-documents"));
copyFileSync(join(operation_dir, "operation-html-artifacts.js"), join(operation_dir, "operation-html-artifacts"));
copyFileSync(join(operation_dir, "operation-image-source.js"), join(operation_dir, "operation-image-source"));
copyFileSync(join(operation_dir, "operation-pending-permissions.js"), join(operation_dir, "operation-pending-permissions"));
copyFileSync(join(operation_dir, "operation-projection-preview.js"), join(operation_dir, "operation-projection-preview"));
copyFileSync(join(operation_dir, "operation-projection-timeline.js"), join(operation_dir, "operation-projection-timeline"));
copyFileSync(join(operation_dir, "operation-types.js"), join(operation_dir, "operation-types"));
copyFileSync(join(operation_dir, "operation-desktop-types.js"), join(operation_dir, "operation-desktop-types"));
copyFileSync(join(operation_dir, "operation-preview.js"), join(operation_dir, "operation-preview"));
copyFileSync(join(operation_dir, "operation-scene-planner-helpers.js"), join(operation_dir, "operation-scene-planner-helpers"));
copyFileSync(join(operation_dir, "operation-scene-focus.js"), join(operation_dir, "operation-scene-focus"));
copyFileSync(join(operation_dir, "operation-scene-window-builder.js"), join(operation_dir, "operation-scene-window-builder"));
copyFileSync(join(operation_dir, "operation-scene-window-policy.js"), join(operation_dir, "operation-scene-window-policy"));
copyFileSync(join(operation_dir, "operation-stage-labels.js"), join(operation_dir, "operation-stage-labels"));
copyFileSync(join(operation_dir, "operation-stage-experience.js"), join(operation_dir, "operation-stage-experience"));
copyFileSync(join(operation_dir, "operation-stage-snapshot-merge.js"), join(operation_dir, "operation-stage-snapshot-merge"));
copyFileSync(join(operation_dir, "operation-stage-key.js"), join(operation_dir, "operation-stage-key"));
copyFileSync(join(operation_dir, "operation-stage-open-command.js"), join(operation_dir, "operation-stage-open-command"));
copyFileSync(join(operation_dir, "operation-terminal-progress.js"), join(operation_dir, "operation-terminal-progress"));
copyFileSync(join(operation_dir, "operation-terminal-lines.js"), join(operation_dir, "operation-terminal-lines"));
copyFileSync(join(operation_dir, "operation-terminal-session-events.js"), join(operation_dir, "operation-terminal-session-events"));
copyFileSync(join(operation_dir, "operation-summary-events.js"), join(operation_dir, "operation-summary-events"));
copyFileSync(join(operation_dir, "operation-event-io.js"), join(operation_dir, "operation-event-io"));
copyFileSync(join(operation_dir, "operation-runtime-event-stream.js"), join(operation_dir, "operation-runtime-event-stream"));
copyFileSync(join(operation_dir, "operation-runtime-types.js"), join(operation_dir, "operation-runtime-types"));
copyFileSync(join(operation_dir, "operation-tool-visual-contract.js"), join(operation_dir, "operation-tool-visual-contract"));
copyFileSync(
  join(shared_editor_dir, "workspace-file-preview-kind.js"),
  join(shared_editor_dir, "workspace-file-preview-kind"),
);
mkdirSync(join(operation_dir, "stage"), { recursive: true });
copyFileSync(join(operation_dir, "stage/operation-stage-window-kinds.js"), join(operation_dir, "stage/operation-stage-window-kinds"));
copyFileSync(join(operation_dir, "stage/operation-stage-event-sequence.js"), join(operation_dir, "stage/operation-stage-event-sequence"));
copyFileSync(join(operation_dir, "stage/operation-stage-dock-model.js"), join(operation_dir, "stage/operation-stage-dock-model"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-actions.js"), join(operation_dir, "stage/operation-stage-window-actions"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-reveal.js"), join(operation_dir, "stage/operation-stage-window-reveal"));
copyFileSync(join(operation_dir, "stage/operation-stage-hidden-windows.js"), join(operation_dir, "stage/operation-stage-hidden-windows"));
copyFileSync(join(operation_dir, "stage/operation-stage-app-identity.js"), join(operation_dir, "stage/operation-stage-app-identity"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-focus.js"), join(operation_dir, "stage/operation-stage-window-focus"));
copyFileSync(join(operation_dir, "stage/operation-stage-keyboard-target.js"), join(operation_dir, "stage/operation-stage-keyboard-target"));
copyFileSync(join(operation_dir, "stage/operation-stage-menu-model.js"), join(operation_dir, "stage/operation-stage-menu-model"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-titlebar.js"), join(operation_dir, "stage/operation-stage-window-titlebar"));
copyFileSync(join(operation_dir, "stage/operation-stage-desktop-icons.js"), join(operation_dir, "stage/operation-stage-desktop-icons"));
copyFileSync(join(operation_dir, "stage/operation-stage-minimized-window.js"), join(operation_dir, "stage/operation-stage-minimized-window"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-drag.js"), join(operation_dir, "stage/operation-stage-window-drag"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-launch.js"), join(operation_dir, "stage/operation-stage-window-launch"));
copyFileSync(join(operation_dir, "stage/operation-stage-window-position.js"), join(operation_dir, "stage/operation-stage-window-position"));
copyFileSync(join(operation_dir, "stage/operation-stage-live-strip.js"), join(operation_dir, "stage/operation-stage-live-strip"));
mkdirSync(join(operation_dir, "apps"), { recursive: true });
copyFileSync(join(operation_dir, "apps/terminal-result-model.js"), join(operation_dir, "apps/terminal-result-model"));
copyFileSync(join(operation_dir, "apps/terminal-session-model.js"), join(operation_dir, "apps/terminal-session-model"));
copyFileSync(join(operation_dir, "apps/operation-app-surface-policy.js"), join(operation_dir, "apps/operation-app-surface-policy"));
copyFileSync(join(operation_dir, "apps/file-preview-value.js"), join(operation_dir, "apps/file-preview-value"));
copyFileSync(join(operation_dir, "apps/code-editor-session.js"), join(operation_dir, "apps/code-editor-session"));
copyFileSync(join(operation_dir, "apps/browser-reader-model.js"), join(operation_dir, "apps/browser-reader-model"));
copyFileSync(join(operation_dir, "apps/browser-result-items.js"), join(operation_dir, "apps/browser-result-items"));
copyFileSync(join(operation_dir, "apps/browser-session.js"), join(operation_dir, "apps/browser-session"));
copyFileSync(join(operation_dir, "apps/finder-item-details.js"), join(operation_dir, "apps/finder-item-details"));
copyFileSync(join(operation_dir, "apps/finder-session.js"), join(operation_dir, "apps/finder-session"));
copyFileSync(join(operation_dir, "apps/run-manifest-console.js"), join(operation_dir, "apps/run-manifest-console"));
copyFileSync(join(operation_dir, "apps/run-manifest-sources.js"), join(operation_dir, "apps/run-manifest-sources"));
copyFileSync(join(operation_dir, "apps/task-app-model.js"), join(operation_dir, "apps/task-app-model"));

const { projectOperationSnapshot } = await import(pathToFileURL(join(operation_dir, "operation-projector.js")));
const { resolveOperationToolProfile } = await import(pathToFileURL(join(operation_dir, "operation-tool-catalog.js")));
const {
  planOperationDesktop,
  resolveOperationEventWindowId,
} = await import(pathToFileURL(join(operation_dir, "operation-scene-planner.js")));
const {
  deriveStageDesktopIntents,
  deriveStageDesktopIntentsFromRuntimeEvent,
  operationEventFromRuntimeEvent,
  readBrowserOpenTargetFromTerminalCommand,
  stageAppSessionIdForIntent,
} = await import(pathToFileURL(join(operation_dir, "operation-desktop-intents.js")));
const {
  buildOperationContinuationBrief,
  buildOperationLiveEpisode,
  deriveOperationStageExperiencePhase,
} = await import(pathToFileURL(join(operation_dir, "operation-stage-experience.js")));
const {
  mergeOperationStageSnapshotsForRestore,
} = await import(pathToFileURL(join(operation_dir, "operation-stage-snapshot-merge.js")));
const {
  buildOperationStageKey,
} = await import(pathToFileURL(join(operation_dir, "operation-stage-key.js")));
const {
  fallbackStageEventObjectLabel,
  fallbackStageEventTargetLabel,
  isLowSignalStageLabel,
} = await import(pathToFileURL(join(operation_dir, "operation-stage-labels.js")));
const {
  OPERATION_TOOL_VISUAL_GROUPS,
  resolveOperationToolVisualContract,
} = await import(pathToFileURL(join(operation_dir, "operation-tool-visual-contract.js")));
const {
  isStageDesktopWindowKind,
  windowContentModeForKind,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-kinds.js")));
const {
  buildDockAppSlots,
  groupDockWindowsByApp,
  resolveDockSlotPresentation,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-dock-model.js")));
const {
  resolveOperationWindowKeyboardAction,
  shouldHandleStageDesktopKeyboardAction,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-actions.js")));
const {
  countDesktopRevealEvents,
  initialRevealedWindowCount,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-reveal.js")));
const {
  summarizeHiddenStageWindows,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-hidden-windows.js")));
const {
  dockIconSkinForKind,
  stageAppLabelForWindowKind,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-app-identity.js")));
const {
  resolveNextWindowFocus,
  resolveCycledWindowFocus,
  resolveStageWindowFocusPhase,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-focus.js")));
const {
  shouldIgnoreStageDesktopKeyboardTarget,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-keyboard-target.js")));
const {
  buildStageMenuStatus,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-menu-model.js")));
const {
  buildStageWindowTitlebarState,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-titlebar.js")));
const {
  buildStageDesktopIconItems,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-desktop-icons.js")));
const {
  buildStageMinimizedWindowTile,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-minimized-window.js")));
const {
  isMeaningfulStageWindowDrag,
  normalizeStageWindowDragOffset,
  normalizeStageWindowResizeSize,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-drag.js")));
const {
  buildStageWindowLaunchState,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-launch.js")));
const {
  isStageBackgroundWindow,
  positionForWindow,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-window-position.js")));
const {
  buildStageLiveStripState,
} = await import(pathToFileURL(join(operation_dir, "stage/operation-stage-live-strip.js")));
const {
  buildTerminalSession,
} = await import(pathToFileURL(join(operation_dir, "apps/terminal-session-model.js")));
const {
  parseTerminalResult,
} = await import(pathToFileURL(join(operation_dir, "apps/terminal-result-model.js")));
const {
  appSurfaceForWindowKind,
} = await import(pathToFileURL(join(operation_dir, "apps/operation-app-surface-policy.js")));
const {
  resolveFilePreviewValue,
} = await import(pathToFileURL(join(operation_dir, "apps/file-preview-value.js")));
const {
  buildCodeEditorSessionView,
} = await import(pathToFileURL(join(operation_dir, "apps/code-editor-session.js")));
const {
  buildBrowserResultItems,
} = await import(pathToFileURL(join(operation_dir, "apps/browser-result-items.js")));
const {
  buildBrowserReaderParagraphs,
} = await import(pathToFileURL(join(operation_dir, "apps/browser-reader-model.js")));
const {
  buildBrowserSessionView,
} = await import(pathToFileURL(join(operation_dir, "apps/browser-session.js")));
const {
  finderFileKindLabel,
  finderPreviewLines,
  resolveFinderSelectedItem,
} = await import(pathToFileURL(join(operation_dir, "apps/finder-item-details.js")));
const {
  buildFinderSessionView,
  workspaceStatusLabel,
} = await import(pathToFileURL(join(operation_dir, "apps/finder-session.js")));
const {
  consoleEventLevel,
  consoleEventSubsystem,
} = await import(pathToFileURL(join(operation_dir, "apps/run-manifest-console.js")));
const {
  collectManifestLogSources,
} = await import(pathToFileURL(join(operation_dir, "apps/run-manifest-sources.js")));
const {
  buildTaskAppSession,
} = await import(pathToFileURL(join(operation_dir, "apps/task-app-model.js")));
const now = Date.now();

verify_desktop_window_kind_contract();
verify_dock_model_groups_windows_by_mac_app();
verify_window_keyboard_actions_match_mac_window_controls();
verify_initial_window_reveal_avoids_desktop_clutter_flash();
verify_hidden_stage_uses_desktop_state_instead_of_mission_control();
verify_unclassified_tool_activity_does_not_open_window(now);
verify_current_unclassified_tool_does_not_steal_existing_app_focus(now);
verify_result_artifact_opens_preview_instead_of_unclassified_window(now);
verify_task_planner_opens_tasks_app(now);
verify_desktop_intents_drive_app_session_windows(now);
verify_runtime_events_drive_app_session_windows(now);
verify_runtime_event_projection(now);
verify_tool_visual_contract_inventory(now);
verify_window_focus_moves_to_next_visible_window();
verify_desktop_keyboard_target_policy();
verify_stage_menu_status_tracks_desktop_windows();
verify_stage_window_titlebar_state();
verify_stage_desktop_icon_items();
verify_stage_minimized_window_tile();
verify_stage_window_drag_model();
verify_stage_window_launch_model();
verify_stage_window_position_model();
verify_stage_live_strip_tracks_current_tool();
verify_operation_stage_key_is_session_scoped();
verify_stage_experience_state_machine(now);
verify_live_episode_narrates_running_round(now);
verify_api_retry_runtime_projection(now);
verify_active_event_stays_with_latest_round(now);
verify_error_summary_settles_live_handoff(now);
verify_stage_restore_merge_preserves_round_context(now);
verify_workspace_live_stays_in_tool_round(now);
verify_multi_file_windows_keep_event_identity(now);
verify_code_writer_preview_uses_real_content(now);
verify_extensionless_workspace_file_opens_code_app(now);
verify_code_editor_session_view();
verify_terminal_result_envelope(now);
verify_nxs_terminal_progress_and_claude_fallback(now);
verify_terminal_entries_render_real_command_result(now);
verify_browser_fallback_builds_search_results(now);
verify_browser_reader_highlights_tool_hits();
verify_browser_session_view(now);
verify_finder_details_reflect_selected_workspace_item(now);
verify_finder_session_view(now);
verify_console_events_use_mac_app_subsystems(now);
verify_tasks_app_uses_real_task_fields(now);
verify_completed_manifest_keeps_terminal_window_identity(now);
verify_completed_round_replay_uses_event_slice({
  assert,
  now,
  planOperationDesktop,
  projectOperationSnapshot,
});
verify_html_artifact_opens_browser_srcdoc({
  assert,
  now,
  planOperationDesktop,
  projectOperationSnapshot,
});
verify_pending_permissions_are_scoped_and_precise(now);
verify_live_round_without_tool_events_stays_hidden(now);
verify_synthetic_error_summary(now);

console.log("operation-stage projector verification passed");

function verify_desktop_window_kind_contract() {
  const expected_desktop_apps = [
    "browser",
    "code_editor",
    "file_preview",
    "finder",
    "handoff",
    "image_viewer",
    "markdown_reader",
    "pdf_reader",
    "permission_wait",
    "presentation",
    "run_manifest",
    "spreadsheet",
    "tasks",
    "terminal",
    "word_reader",
  ];
  for (const kind of expected_desktop_apps) {
    assert(isStageDesktopWindowKind(kind), `${kind} should be rendered as a desktop app window`);
  }
  for (const kind of ["evidence", "summary"]) {
    assert(!isStageDesktopWindowKind(kind), `${kind} should not render as a standalone desktop app window`);
  }
  for (const kind of expected_desktop_apps.filter((kind) => kind !== "permission_wait")) {
    assert(windowContentModeForKind(kind) === "flush", `${kind} should fill its app window content area`);
  }
  assert(windowContentModeForKind("permission_wait") === "inset", "permission wait should keep inset content as a system prompt");
}

function verify_dock_model_groups_windows_by_mac_app() {
  const windows = [
    mock_stage_window({ id: "browser:a", kind: "browser", phase: "background" }),
    mock_stage_window({ id: "browser:b", kind: "browser", phase: "focused" }),
    mock_stage_window({ id: "code:a", kind: "code_editor", phase: "closed" }),
    mock_stage_window({ id: "preview:a", kind: "file_preview", phase: "minimized", title: "legacy.doc" }),
  ];
  const groups = groupDockWindowsByApp(windows, "browser:b", stageAppLabelForWindowKind);
  const Navi_group = groups.find((group) => group.app_label === "Navi");
  assert(Navi_group?.count === 2, `Dock should group Navi windows, got ${Navi_group?.count}`);
  assert(Navi_group?.is_active, "Dock Navi group should be active when one Navi window is focused");
  assert(Navi_group?.window.id === "browser:b", `Dock should keep the focused Navi window, got ${Navi_group?.window.id}`);
  const code_group = groups.find((group) => group.app_label === "Editor");
  assert(code_group?.count === 0, `Dock should not count closed Editor windows as running, got ${code_group?.count}`);
  assert(!code_group?.is_running, "Dock should mark closed Editor window as not running");

  const slots = buildDockAppSlots(groups);
  assert(!slots.some((slot) => slot.window === null), "Dock should only show apps backed by real tool windows");
  assert(slots[0].app_label === "Navi" && slots[0].count === 2, "Dock Navi slot should reflect grouped running windows");
  assert(slots[1].app_label === "Editor" && slots[1].window?.id === "code:a", "Dock Editor slot should keep recoverable closed window");
  assert(slots.at(-1)?.app_label === "预览", `Dock should append unpinned running apps, got ${slots.at(-1)?.app_label}`);

  const Navi_presentation = resolveDockSlotPresentation(slots[0], "Search");
  assert(Navi_presentation.state === "active", `Dock active Navi slot should present as active, got ${Navi_presentation.state}`);
  assert(Navi_presentation.title === "Navi · 2 个窗口 · 当前", `Dock active Navi title should summarize grouped windows, got ${Navi_presentation.title}`);
  const code_presentation = resolveDockSlotPresentation(slots[1], "app.ts");
  assert(code_presentation.state === "recoverable", `Dock closed Editor slot should be recoverable, got ${code_presentation.state}`);
  assert(!code_presentation.is_disabled, "Dock closed Editor slot should remain clickable for restore");
  const preview_slot = slots.at(-1);
  assert(preview_slot, "Dock should keep the minimized Preview app slot");
  const preview_presentation = resolveDockSlotPresentation(preview_slot, "legacy.doc");
  assert(preview_presentation.state === "minimized", `Dock minimized Preview slot should present as minimized, got ${preview_presentation.state}`);
  assert(preview_presentation.title === "预览 · legacy.doc · 已最小化", `Dock should preserve the real legacy preview title, got ${preview_presentation.title}`);
}

function verify_window_keyboard_actions_match_mac_window_controls() {
  assert(resolveOperationWindowKeyboardAction({ key: "Enter" }) === "focus", "Enter should focus a desktop window");
  assert(resolveOperationWindowKeyboardAction({ key: " " }) === "focus", "Space should focus a desktop window");
  assert(resolveOperationWindowKeyboardAction({ key: "Escape" }) === "minimize", "Escape should minimize the focused window");
  assert(resolveOperationWindowKeyboardAction({ key: "w", metaKey: true }) === "close", "Cmd+W should close the focused window");
  assert(resolveOperationWindowKeyboardAction({ key: "M", metaKey: true }) === "minimize", "Cmd+M should minimize the focused window");
  assert(resolveOperationWindowKeyboardAction({ key: "f", metaKey: true, ctrlKey: true }) === "zoom", "Ctrl+Cmd+F should zoom the focused window");
  assert(resolveOperationWindowKeyboardAction({ key: "Enter", metaKey: true }) === "zoom", "Cmd+Enter should zoom the focused window");
  assert(resolveOperationWindowKeyboardAction({ key: "`", metaKey: true }) === "cycle_next", "Cmd+` should cycle to the next desktop window");
  assert(resolveOperationWindowKeyboardAction({ key: "`", metaKey: true, shiftKey: true }) === "cycle_previous", "Cmd+Shift+` should cycle to the previous desktop window");
  assert(resolveOperationWindowKeyboardAction({ key: "w", metaKey: true, shiftKey: true }) === null, "Modified Cmd+W should not trigger the simple close action");
  assert(resolveOperationWindowKeyboardAction({ key: "a", metaKey: true }) === null, "Unrelated shortcuts should stay with the app content");
  assert(!shouldHandleStageDesktopKeyboardAction("focus"), "Desktop-level shortcuts should not hijack Enter or Space focus behavior");
  assert(shouldHandleStageDesktopKeyboardAction("cycle_next"), "Desktop-level shortcuts should handle window cycling");
  assert(shouldHandleStageDesktopKeyboardAction("close"), "Desktop-level shortcuts should handle active window closing");
}

function verify_initial_window_reveal_avoids_desktop_clutter_flash() {
  assert(countDesktopRevealEvents([
    { id: "wake", surface: "conversation" },
    { id: "write", surface: "editor", tool_use_id: "tool-write" },
    { id: "open", surface: "terminal", tool_use_id: "tool-open" },
  ]) === 2, "Desktop reveal should count tool/application events instead of the initial empty desktop wake event");
  assert(initialRevealedWindowCount({
    minimum_count: 1,
    phase: "running",
    window_count: 5,
  }) === 1, "Running stage should reveal only the first window on the first paint");
  assert(initialRevealedWindowCount({
    minimum_count: 2,
    phase: "awakening",
    window_count: 5,
  }) === 2, "Awakening stage should respect the minimum narrative window count");
  assert(initialRevealedWindowCount({
    minimum_count: 1,
    phase: "completed",
    window_count: 5,
  }) === 5, "Completed stage should reveal the full review desktop immediately");
  assert(initialRevealedWindowCount({
    minimum_count: 1,
    phase: "running",
    window_count: 0,
  }) === 0, "Empty desktop should stay empty on the first paint");
}

function verify_hidden_stage_uses_desktop_state_instead_of_mission_control() {
  const minimized_summary = summarizeHiddenStageWindows([
    mock_stage_window({ id: "hidden:terminal", kind: "terminal", phase: "minimized" }),
    mock_stage_window({ id: "hidden:browser", kind: "browser", phase: "minimized" }),
  ]);
  assert(minimized_summary.hidden_count === 2, `Hidden summary should count hidden windows, got ${minimized_summary.hidden_count}`);
  assert(minimized_summary.label === "2 个窗口在 Dock", `Minimized desktop should point users to Dock, got ${minimized_summary.label}`);

  const mixed_summary = summarizeHiddenStageWindows([
    mock_stage_window({ id: "hidden:terminal", kind: "terminal", phase: "minimized" }),
    mock_stage_window({ id: "hidden:browser", kind: "browser", phase: "closed" }),
  ]);
  assert(mixed_summary.label === "1 个在 Dock · 1 个已关闭", `Mixed hidden desktop should avoid Mission Control language, got ${mixed_summary.label}`);
  assert(!mixed_summary.label.toLowerCase().includes("mission"), "Hidden desktop summary should not use Mission Control panel language");
}

function verify_unclassified_tool_activity_does_not_open_window(now) {
  const event = {
    id: "tool-context-docs",
    session_key: "session:stage",
    round_id: "round-unclassified-tool",
    agent_id: "agent-stage",
    tool_use_id: "tool-context",
    tool_name: "Context7",
    kind: "unknown",
    surface: "fallback",
    phase: "running",
    title: "查询文档",
    target: "React cleanup",
    input_preview: {
      library: "react",
      query: "useEffect cleanup",
    },
    updated_at: now,
  };
  const desktop = planOperationDesktop({
    event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: event,
      events: [event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  });
  assert(desktop.windows.length === 0, `Unclassified tool activity should stay in execution path only, got ${desktop.windows.length} windows`);
  assert(desktop.active_window_id === null, "Unclassified tool activity should not create an active desktop window");
}

function verify_current_unclassified_tool_does_not_steal_existing_app_focus(now) {
  const read_event = {
    id: "tool-read",
    session_key: "session:stage",
    round_id: "round-mixed-tools",
    agent_id: "agent-stage",
    tool_use_id: "tool-read",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "workspace",
    phase: "done",
    title: "读取文件",
    target: "/workspace/app.ts",
    input_preview: {
      file_path: "/workspace/app.ts",
    },
    result_preview: "export const app = true;",
    updated_at: now - 10,
  };
  const generic_event = {
    id: "tool-context-docs",
    session_key: "session:stage",
    round_id: "round-mixed-tools",
    agent_id: "agent-stage",
    tool_use_id: "tool-context",
    tool_name: "Context7",
    kind: "unknown",
    surface: "fallback",
    phase: "running",
    title: "查询文档",
    target: "React cleanup",
    input_preview: {
      library: "react",
      query: "useEffect cleanup",
    },
    updated_at: now,
  };
  const desktop = planOperationDesktop({
    event: generic_event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: generic_event,
      events: [read_event, generic_event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  });
  const code_window = desktop.windows.find((window) => window.kind === "code_editor");
  assert(code_window, "Existing document app window should remain on the desktop");
  assert(desktop.active_window_id === code_window.id, "Unclassified tool should not steal focus from the existing document app");
}

function verify_result_artifact_opens_preview_instead_of_unclassified_window(now) {
  const event = {
    id: "tool-imagegen",
    session_key: "session:stage",
    round_id: "round-imagegen",
    agent_id: "agent-stage",
    tool_use_id: "tool-image",
    tool_name: "mcp__nexus_imagegen__generate_image",
    kind: "unknown",
    surface: "fallback",
    phase: "done",
    title: "生成图片",
    target: "A cute fluffy kitten",
    result_preview: {
      action: "generate_image",
      item: {
        markdown: "![generated image](output/imagegen/cute-kitten.png)",
      },
    },
    evidence: [{
      label: "generated image",
      type: "artifact",
      value: "output/imagegen/cute-kitten.png",
    }],
    updated_at: now,
  };
  const desktop = planOperationDesktop({
    event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: event,
      events: [event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  });
  const preview_window = desktop.windows.find((window) => window.kind === "image_viewer");
  assert(preview_window, "Tool result image artifacts should open in Preview instead of the generic tool app");
  assert(preview_window.target === "output/imagegen/cute-kitten.png", `Preview should target the generated image, got ${preview_window?.target}`);
  assert(desktop.windows.length === 1, `Result-backed artifact tools should only open the preview app, got ${desktop.windows.length} windows`);
  assert(desktop.active_window_id === preview_window.id, "Generated image Preview window should become active");
  assert(resolveOperationEventWindowId(event, desktop.windows) === preview_window.id, "Original image tool event should focus its Preview window");
}

function verify_task_planner_opens_tasks_app(now) {
  const event = {
    id: "tool-todo-write",
    session_key: "session:stage",
    round_id: "round-task-planner",
    agent_id: "agent-stage",
    tool_use_id: "tool-todo",
    tool_name: "TodoWrite",
    kind: "plan_update",
    surface: "task",
    phase: "running",
    title: "更新计划",
    target: "todos",
    input_preview: {
      todos: [{ content: "运行验证", status: "in_progress" }],
    },
    updated_at: now,
  };
  const desktop = planOperationDesktop({
    event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: event,
      events: [event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  });
  const task_window = desktop.windows.find((window) => window.kind === "tasks");
  assert(task_window, "TodoWrite should open Tasks instead of a generic text card");
  assert(task_window.id === "round-task-planner:tasks", `Tasks session id should be stable per round, got ${task_window.id}`);
  assert(desktop.active_window_id === task_window.id, "Current TodoWrite activity window should be focused");
}

function verify_desktop_intents_drive_app_session_windows(now) {
  const terminal_event = {
    id: "tool-bash-open-html",
    session_key: "session:stage",
    round_id: "round-intents",
    agent_id: "agent-stage",
    message_id: "msg-intents",
    tool_use_id: "tool-bash",
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    phase: "done",
    title: "打开预览",
    target: "open gomoku.html",
    input_preview: {
      command: "open gomoku.html",
    },
    result_preview: {
      content: "Opening gomoku.html\nNavi preview launched\n",
      exit_code: 0,
      is_error: false,
    },
    updated_at: now,
  };
  const terminal_intents = deriveStageDesktopIntents(terminal_event);
  assert(terminal_intents.some((intent) => intent.app === "terminal" && intent.action === "run_command"), "Bash should derive a Terminal desktop intent");
  assert(terminal_intents.some((intent) => intent.app === "browser" && intent.action === "preview_artifact"), "Bash open html should derive a Navi preview intent");
  const waiting_intents = deriveStageDesktopIntents({
    ...terminal_event,
    phase: "waiting",
    permission_request_id: "permission-open-html",
  });
  assert(waiting_intents.some((intent) => intent.app === "terminal"), "Waiting Bash should keep the Terminal intent");
  assert(!waiting_intents.some((intent) => intent.app === "browser"), "Waiting Bash must not launch Navi before approval");
  const waiting_event = {
    ...terminal_event,
    phase: "waiting",
    permission_request_id: "permission-open-html",
  };
  const waiting_desktop = planOperationDesktop({
    event: waiting_event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: waiting_event,
      events: [waiting_event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  });
  assert(waiting_desktop.windows.find((window) => window.kind === "terminal")?.phase === "focused", "Waiting Bash should keep Terminal focused");
  assert(!waiting_desktop.windows.some((window) => window.kind === "browser"), "Waiting Bash must not create a browser window");
  const open_target = readBrowserOpenTargetFromTerminalCommand(terminal_event);
  assert(open_target?.target === "gomoku.html", `open html command should expose the artifact target, got ${open_target?.target}`);
  const redirected_payload = Buffer.from(JSON.stringify({
    command: "open '/workspace/reports/weekly brief.md'",
    target: "reports/weekly brief.md",
  })).toString("base64url");
  const redirected_event = {
    ...terminal_event,
    id: "tool-bash-open-markdown",
    tool_use_id: "tool-bash-markdown",
    target: "reports/weekly brief.md",
    input_preview: {
      command: `: # __NEXUS_STAGE_OPEN_V1__${redirected_payload}`,
    },
  };
  const redirected_target = readBrowserOpenTargetFromTerminalCommand(redirected_event);
  assert(redirected_target?.target === "reports/weekly brief.md", `stage redirect marker should restore the relative target, got ${redirected_target?.target}`);
  const redirected_intents = deriveStageDesktopIntents(redirected_event);
  assert(redirected_intents.some((intent) => intent.app === "preview"), "non-html open should derive a real workspace preview intent");
  assert(!redirected_intents.some((intent) => intent.app === "browser"), "non-html open must not launch Navi");
  const redirected_desktop = planOperationDesktop({
    event: redirected_event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: redirected_event,
      events: [redirected_event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [{
        id: "workspace-weekly-brief",
        agent_id: "agent-stage",
        path: "reports/weekly brief.md",
        status: "updated",
        version: 1,
        source: "agent",
        session_key: "session:stage",
        tool_use_id: "tool-bash-markdown",
        event_type: "file_write_end",
        live_content: "# Weekly brief",
        updated_at: now,
      }],
      updated_at: now,
    },
  });
  const preview_window = redirected_desktop.windows.find((window) => window.kind === "markdown_reader");
  assert(preview_window?.payload.workspace_preview === true, "open markdown should use the shared workspace preview implementation");
  assert(preview_window?.phase === "focused", `open markdown should focus its document window, got ${preview_window?.phase}`);
  assert(
    readBrowserOpenTargetFromTerminalCommand({
      ...terminal_event,
      input_preview: { command: "cat gomoku.html" },
      target: "cat gomoku.html",
    }) === null,
    "Mentioning an html file in a non-open command should not launch Navi",
  );

  const terminal_session_id = stageAppSessionIdForIntent(
    terminal_event.round_id,
    terminal_intents.find((intent) => intent.app === "terminal"),
    (value) => value,
  );
  assert(terminal_session_id === "round-intents:terminal", `Terminal session id should be stable per round, got ${terminal_session_id}`);

  const web_search_event = {
    ...terminal_event,
    id: "tool-web-search",
    tool_use_id: "tool-web-search",
    tool_name: "web_search",
    kind: "web_research",
    surface: "web",
    title: "搜索资料",
    target: "macOS Navi window chrome",
    input_preview: {
      query: "macOS Navi window chrome",
    },
    result_preview: [{
      title: "Navi browser chrome",
      url: "https://example.com/navi",
      snippet: "Navi keeps a focused address field and toolbar.",
    }],
  };
  const web_fetch_event = {
    ...web_search_event,
    id: "tool-web-fetch",
    tool_use_id: "tool-web-fetch",
    tool_name: "fetch",
    target: "https://example.com/navi",
    input_preview: {
      url: "https://example.com/navi",
    },
    updated_at: now + 1,
  };
  const desktop = planOperationDesktop({
    event: web_fetch_event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: web_fetch_event,
      events: [web_search_event, web_fetch_event],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now + 1,
    },
  });
  const browser_window = desktop.windows.find((window) => window.kind === "browser");
  assert(browser_window?.id === "round-intents:browser", `Navi should use one stable browser app session, got ${browser_window?.id}`);
  assert(browser_window.payload.related_events.length === 2, `Navi session should keep web_search and fetch history, got ${browser_window.payload.related_events.length}`);
  assert(browser_window.phase === "focused", `Latest web event should focus Navi, got ${browser_window.phase}`);
}

function verify_runtime_events_drive_app_session_windows(now) {
  const runtime_cases = [
    {
      event: {
        id: "runtime:read:start",
        event_type: "tool_start",
        session_key: "session:stage",
        round_id: "round-runtime",
        agent_id: "agent-stage",
        tool_use_id: "tool-read",
        tool_name: "Read",
        phase: "running",
        timestamp: now,
        input: { file_path: "src/app.ts" },
      },
      expected_app: "code",
      expected_session: "round-runtime:document:src/app.ts",
    },
    {
      event: {
        id: "runtime:bash:start",
        event_type: "tool_start",
        session_key: "session:stage",
        round_id: "round-runtime",
        agent_id: "agent-stage",
        tool_use_id: "tool-bash",
        tool_name: "Bash",
        phase: "running",
        timestamp: now,
        input: { command: "pnpm --dir web typecheck" },
      },
      expected_app: "terminal",
      expected_session: "round-runtime:terminal",
    },
    {
      event: {
        id: "runtime:web:start",
        event_type: "tool_start",
        session_key: "session:stage",
        round_id: "round-runtime",
        agent_id: "agent-stage",
        tool_use_id: "tool-web",
        tool_name: "WebFetch",
        phase: "running",
        timestamp: now,
        input: { url: "https://example.com" },
      },
      expected_app: "browser",
      expected_session: "round-runtime:browser",
    },
    {
      event: {
        id: "runtime:permission:request",
        event_type: "permission_request",
        session_key: "session:stage",
        round_id: "round-runtime",
        agent_id: "agent-stage",
        tool_use_id: "tool-write",
        tool_name: "Write",
        phase: "waiting",
        timestamp: now,
        input: { file_path: "src/app.ts" },
        permission_request_id: "permission-write",
      },
      expected_app: "system",
      expected_session: "round-runtime:system-gate",
    },
  ];

  for (const test_case of runtime_cases) {
    const intents = deriveStageDesktopIntentsFromRuntimeEvent(test_case.event);
    const intent = intents.find((item) => item.app === test_case.expected_app);
    assert(intent, `${test_case.event.event_type}:${test_case.event.tool_name} should derive ${test_case.expected_app} intent`);
    const session_id = stageAppSessionIdForIntent(test_case.event.round_id, intent, (value) => value);
    assert(session_id === test_case.expected_session, `Runtime event should map to stable ${test_case.expected_app} session, got ${session_id}`);
  }
}

function verify_runtime_event_projection(now) {
  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    live_round_ids: ["round-runtime-projection"],
    pending_permissions: [{
      request_id: "permission-bash",
      tool_name: "Bash",
      tool_input: { command: "open report.html" },
      session_key: "session:stage",
      agent_id: "agent-stage",
      message_id: "msg-runtime",
      caused_by: "round-runtime-projection",
      interaction_mode: "permission",
      risk_label: "需要确认",
      summary: "允许打开本地 HTML",
    }],
    workspace_events: [{
      agent_id: "agent-stage",
      event_type: "file_write_end",
      id: "workspace-report-html",
      live_content: "<!doctype html><title>Report</title>",
      path: "report.html",
      session_key: "session:stage",
      source: "agent",
      status: "updated",
      tool_use_id: "tool-write",
      updated_at: now + 10,
      version: 1,
    }],
    messages: [
      {
        role: "user",
        message_id: "msg-user-runtime",
        session_key: "session:stage",
        agent_id: "agent-stage",
        round_id: "round-runtime-projection",
        content: "生成并打开 report.html",
        timestamp: now - 30,
      },
      {
        role: "assistant",
        message_id: "msg-runtime",
        session_key: "session:stage",
        agent_id: "agent-stage",
        round_id: "round-runtime-projection",
        timestamp: now,
        is_complete: false,
        content: [
          {
            type: "tool_use",
            id: "tool-bash",
            name: "Bash",
            input: { command: "open report.html" },
          },
          {
            type: "tool_use",
            id: "tool-read",
            name: "Read",
            input: { file_path: "report.html" },
          },
          {
            type: "tool_use",
            id: "tool-bash-running",
            name: "Bash",
            input: { command: "sleep 2" },
          },
          {
            type: "task_progress",
            task_id: "tool-bash-running",
            tool_use_id: "tool-bash-running",
            last_tool_name: "Bash",
            description: "Bash 正在执行",
            usage: { duration_ms: 1200 },
          },
        ],
      },
      {
        role: "assistant",
        message_id: "msg-summary-runtime",
        session_key: "session:stage",
        agent_id: "agent-stage",
        round_id: "round-runtime-projection",
        timestamp: now + 20,
        is_complete: true,
        content: [
          {
            type: "tool_result",
            tool_use_id: "tool-read",
            content: "<!doctype html><title>Report</title>",
          },
        ],
        result_summary: {
          subtype: "success",
          duration_ms: 1200,
          duration_api_ms: 800,
          num_turns: 2,
          result: "report.html 已打开",
          is_error: false,
        },
      },
    ],
  });
  const event_types = new Set(snapshot.runtime_events.map((event) => event.event_type));
  for (const event_type of ["tool_start", "tool_delta", "tool_end", "artifact_update", "permission_request", "round_handoff"]) {
    assert(event_types.has(event_type), `Runtime projection should include ${event_type}`);
  }
  assert(snapshot.runtime_events.some((event) => (
    event.event_type === "permission_request" &&
    event.permission_request_id === "permission-bash" &&
    event.tool_use_id === "tool-bash"
  )), "Runtime permission request should preserve request id and tool_use_id");
  assert(snapshot.runtime_events.some((event) => (
    event.event_type === "artifact_update" &&
    event.tool_use_id === "tool-write" &&
    event.artifact?.path === "report.html"
  )), "Runtime artifact update should preserve tool_use_id and workspace path");
}

function verify_tool_visual_contract_inventory(now) {
  const current_tools = new Set([
    "Task",
    "TaskOutput",
    "Bash",
    "KillShell",
    "Glob",
    "Grep",
    "LS",
    "Read",
    "FileRead",
    "ViewImage",
    "Edit",
    "FileEdit",
    "MultiEdit",
    "Write",
    "FileWrite",
    "NotebookEdit",
    "WebFetch",
    "WebSearch",
    "Skill",
    "TodoWrite",
    "EnterPlanMode",
    "ExitPlanMode",
    "AskUserQuestion",
  ]);
  const grouped_tools = new Set(Object.values(OPERATION_TOOL_VISUAL_GROUPS).flatMap((group) => group.tools));
  for (const tool_name of current_tools) {
    assert(grouped_tools.has(tool_name), `${tool_name} should be assigned to a visual tool group`);
  }
  assert(resolveOperationToolProfile("functions.Bash").action === "run", "Wrapped Bash should map to Terminal exactly");
  assert(resolveOperationToolProfile("functions.KillShell").action === "stop", "Wrapped KillShell should map to Terminal exactly");
  assert(resolveOperationToolProfile("cancel_booking").action === "generic", "Unrelated cancel tools must not map to KillShell");
  assert(resolveOperationToolProfile("terminal_status").action === "generic", "Terminal-like names must not map to Bash");

  const base_event = {
    agent_id: "agent-stage",
    id: "visual-contract",
    message_id: "msg-visual",
    phase: "running",
    round_id: "round-visual",
    session_key: "session:stage",
    target: "target",
    title: "visual",
    updated_at: now,
  };
  const cases = [
    { expected_component: "code_writer", expected_group: "workspace_writer", kind: "workspace_edit", surface: "editor", tool_name: "Write" },
    { expected_component: "code_reader", expected_group: "workspace_reader", kind: "workspace_read", surface: "editor", tool_name: "Read" },
    { expected_component: "image_viewer", expected_group: "image_viewer", kind: "workspace_read", surface: "workspace", tool_name: "ViewImage" },
    { expected_component: "terminal", expected_group: "command_runner", kind: "command_run", surface: "terminal", tool_name: "Bash" },
    { expected_component: "browser", expected_group: "web_browser", kind: "web_research", surface: "web", tool_name: "WebSearch" },
    { expected_component: "tasks", expected_group: "task_planner", kind: "plan_update", surface: "task", tool_name: "TodoWrite" },
    { expected_component: "system_gate", expected_group: "human_gate", kind: "human_gate", surface: "conversation", tool_name: "AskUserQuestion" },
    { expected_component: "execution_path", expected_group: "unclassified_action", kind: "unknown", surface: "fallback", tool_name: "Rules" },
  ];

  for (const test_case of cases) {
    const contract = resolveOperationToolVisualContract({
      ...base_event,
      kind: test_case.kind,
      surface: test_case.surface,
      tool_name: test_case.tool_name,
    });
    assert(contract.group === test_case.expected_group, `${test_case.tool_name} should use ${test_case.expected_group}, got ${contract.group}`);
    assert(contract.component === test_case.expected_component, `${test_case.tool_name} should render ${test_case.expected_component}, got ${contract.component}`);
    assert(contract.common_controls.includes("window_drag"), `${test_case.tool_name} should keep shared window drag control`);
  }

  const gate_contract = resolveOperationToolVisualContract({
    ...base_event,
    kind: "human_gate",
    surface: "conversation",
    tool_name: "AskUserQuestion",
  });
  assert(gate_contract.common_controls.includes("confirm"), "human gate should expose confirm control");
  assert(gate_contract.common_controls.includes("deny"), "human gate should expose deny control");
  const gate_intents = deriveStageDesktopIntents({
    ...base_event,
    kind: "human_gate",
    surface: "conversation",
    tool_name: "AskUserQuestion",
  });
  assert(gate_intents.some((intent) => intent.app === "system" && intent.action === "request_confirmation"), "human gate should derive a system confirmation intent");
}

function verify_window_focus_moves_to_next_visible_window() {
  const windows = [
    mock_stage_window({ id: "finder", kind: "finder", phase: "background", z: 12 }),
    mock_stage_window({ id: "browser", kind: "browser", phase: "focused", z: 40 }),
    mock_stage_window({ id: "terminal", kind: "terminal", phase: "background", z: 24 }),
    mock_stage_window({ id: "code", kind: "code_editor", phase: "minimized", z: 36 }),
  ];
  assert(resolveNextWindowFocus({
    current_focus_id: "terminal",
    hidden_window_id: "browser",
    windows,
  }) === "terminal", "Hiding another window should preserve the current focused window");
  assert(resolveNextWindowFocus({
    current_focus_id: "browser",
    hidden_window_id: "browser",
    windows,
  }) === "terminal", "Hiding the focused window should focus the topmost visible replacement");
  assert(resolveNextWindowFocus({
    current_focus_id: "browser",
    hidden_window_id: "terminal",
    windows: windows.map((window) => window.id === "browser" ? { ...window, phase: "minimized" } : window),
  }) === "finder", "Focus fallback should skip minimized windows");
  assert(resolveCycledWindowFocus({
    current_focus_id: "browser",
    direction: "next",
    windows,
  }) === "terminal", "Window cycle should move to the next visible window by z order");
  assert(resolveCycledWindowFocus({
    current_focus_id: "browser",
    direction: "previous",
    windows,
  }) === "finder", "Reverse window cycle should wrap to the previous visible window by z order");
  assert(resolveCycledWindowFocus({
    current_focus_id: null,
    direction: "next",
    windows,
  }) === "browser", "Window cycle should start from the topmost visible window when focus is empty");
  assert(
    resolveStageWindowFocusPhase(windows[1], "terminal") === "background",
    "The previous planned focus should become background after a manual app switch",
  );
  assert(
    resolveStageWindowFocusPhase(windows[2], "terminal") === "focused",
    "The manually selected window should become the only focused app",
  );
  assert(
    resolveStageWindowFocusPhase(windows[3], "code", { restored: true }) === "focused",
    "Restoring a minimized window from the Dock should open it directly in the foreground",
  );
  assert(
    resolveStageWindowFocusPhase(windows[3], "terminal", { restored: true }) === "background",
    "A restored window should join the real background stack when another app owns focus",
  );
  const legacy_preview = mock_stage_window({ id: "legacy", kind: "file_preview", phase: "minimized", title: "legacy.doc" });
  assert(
    resolveStageWindowFocusPhase(legacy_preview, "legacy", { restored: true }) === "focused",
    "Restoring a legacy file preview from the Dock should focus the same Preview window",
  );
}

function verify_desktop_keyboard_target_policy() {
  assert(shouldIgnoreStageDesktopKeyboardTarget({ tag_name: "input" }), "Desktop shortcuts should ignore text inputs");
  assert(shouldIgnoreStageDesktopKeyboardTarget({ tag_name: "textarea" }), "Desktop shortcuts should ignore textareas");
  assert(shouldIgnoreStageDesktopKeyboardTarget({ tag_name: "div", is_content_editable: true }), "Desktop shortcuts should ignore contenteditable areas");
  assert(!shouldIgnoreStageDesktopKeyboardTarget({ tag_name: "button" }), "Desktop shortcuts should still work from window controls and desktop buttons");
  assert(!shouldIgnoreStageDesktopKeyboardTarget({ tag_name: "div" }), "Desktop shortcuts should work from the desktop frame");
}

function verify_stage_menu_status_tracks_desktop_windows() {
  const windows = [
    mock_stage_window({ id: "terminal", kind: "terminal", phase: "focused", title: "open gomoku.html" }),
    mock_stage_window({ id: "browser", kind: "browser", phase: "background" }),
    mock_stage_window({ id: "code", kind: "code_editor", phase: "minimized" }),
    mock_stage_window({ id: "finder", kind: "finder", phase: "closed" }),
  ];
  const status = buildStageMenuStatus(windows, windows[0], (window) => ({
    browser: "Navi",
    code_editor: "Code",
    finder: "文件",
    terminal: "终端",
  })[window.kind] ?? "Nexus");
  assert(status.active_app_label === "终端", `Menu bar should expose the foreground app, got ${status.active_app_label}`);
  assert(status.active_window_label === "open gomoku.html", `Menu bar should expose foreground document title, got ${status.active_window_label}`);
  assert(status.activity_label === "终端 · open gomoku.html", `Menu bar should connect app and focused window, got ${status.activity_label}`);
  assert(status.window_label === "2 个窗口", `Menu bar should count visible app windows, got ${status.window_label}`);
  assert(status.dock_label === "1 个在 Dock", `Menu bar should count minimized windows, got ${status.dock_label}`);

  const idle_status = buildStageMenuStatus([], null, () => "Nexus");
  assert(idle_status.active_window_label === null, `Idle menu bar should not expose a window title, got ${idle_status.active_window_label}`);
  assert(idle_status.activity_label === "桌面待命", `Idle menu bar should report standby, got ${idle_status.activity_label}`);
  assert(idle_status.window_label === "0 个窗口", `Idle menu bar should report zero windows, got ${idle_status.window_label}`);
  assert(idle_status.dock_label === null, `Idle menu bar should omit Dock count, got ${idle_status.dock_label}`);
}

function verify_stage_window_titlebar_state() {
  const focused = buildStageWindowTitlebarState({
    app_label: "Navi",
    focused: true,
    maximized: false,
    minimized: false,
    title: "gomoku.html",
  });
  assert(focused.aria_label === "Navi window: gomoku.html", `Focused titlebar should expose app window label, got ${focused.aria_label}`);
  assert(focused.proxy_label === "Navi", `Focused titlebar should expose a macOS proxy label, got ${focused.proxy_label}`);
  assert(focused.state_dot_tone === "active", `Focused titlebar should use active state dot, got ${focused.state_dot_tone}`);
  assert(focused.status_label === "前台", `Focused titlebar should report foreground status, got ${focused.status_label}`);
  assert(focused.zoom_label === "缩放 gomoku.html", `Focused titlebar should expose zoom action, got ${focused.zoom_label}`);

  const background = buildStageWindowTitlebarState({
    focused: false,
    maximized: true,
    minimized: false,
    title: "Nexus Console",
  });
  assert(background.aria_label === "Nexus Console", `Titlebar without app label should keep plain aria label, got ${background.aria_label}`);
  assert(background.proxy_label === null, `Titlebar without app label should omit proxy label, got ${background.proxy_label}`);
  assert(background.state_dot_tone === "background", `Background titlebar should use background state dot, got ${background.state_dot_tone}`);
  assert(background.status_label === "后台", `Background titlebar should report background status, got ${background.status_label}`);
  assert(background.zoom_title === "还原窗口", `Maximized titlebar should offer restore, got ${background.zoom_title}`);

  const minimized = buildStageWindowTitlebarState({
    app_label: "Code",
    focused: false,
    maximized: false,
    minimized: true,
    title: "app.ts",
  });
  assert(minimized.state_dot_tone === "minimized", `Minimized titlebar should use minimized state dot, got ${minimized.state_dot_tone}`);
  assert(minimized.state_dot_title === "app.ts · 已最小化", `Minimized titlebar should expose state dot title, got ${minimized.state_dot_title}`);
  assert(minimized.status_label === "已最小化", `Minimized titlebar should report minimized status, got ${minimized.status_label}`);
}

function verify_stage_desktop_icon_items() {
  const windows = [
    mock_stage_window({ id: "html", kind: "code_editor", phase: "focused", target: "/workspace/gomoku.html" }),
    mock_stage_window({ id: "md", kind: "markdown_reader", phase: "minimized", target: "/workspace/notes.md" }),
    mock_stage_window({ id: "terminal", kind: "terminal", phase: "focused", target: "pnpm dev" }),
    mock_stage_window({ id: "preview", kind: "code_editor", phase: "background", target: "preview" }),
    mock_stage_window({ id: "png", kind: "image_viewer", phase: "closed", target: "/workspace/screen.png" }),
  ];
  const icons = buildStageDesktopIconItems(windows);
  assert(icons.length === 2, `Desktop should expose only background artifact file windows, got ${icons.length}`);
  assert(icons[0].label === "notes.md", `Desktop icon should skip the foreground file and use basename label, got ${icons[0].label}`);
  assert(icons[0].extension_label === "MD", `Desktop icon should expose a file extension badge, got ${icons[0].extension_label}`);
  assert(icons[0].file_kind_label === "文稿", `Desktop markdown icon should be document kind, got ${icons[0].file_kind_label}`);
  assert(icons[0].state_label === "窗口已最小化", `Minimized desktop icon should expose minimized state, got ${icons[0].state_label}`);
  assert(icons[0].aria_label === "恢复文件窗口：notes.md", `Minimized desktop icon should be a restore action, got ${icons[0].aria_label}`);
  assert(icons[1].file_kind_label === "图像", `Image desktop icon should expose image kind, got ${icons[1].file_kind_label}`);
  assert(icons[1].extension_label === "PNG", `Image desktop icon should expose image extension badge, got ${icons[1].extension_label}`);
  assert(!icons.some((item) => item.label === "preview"), "Desktop should not render synthetic preview artifact icons");
}

function verify_stage_minimized_window_tile() {
  const tile = buildStageMinimizedWindowTile({
    app_label: "Navi",
    title: "gomoku.html",
  });
  assert(tile.aria_label === "从 Dock 恢复：gomoku.html", `Minimized Dock tile should expose restore action, got ${tile.aria_label}`);
  assert(tile.title === "Navi · gomoku.html · 已最小化", `Minimized Dock tile title should include app and state, got ${tile.title}`);
}

function mock_stage_window({
  id,
  kind,
  phase,
  target = id,
  title = id,
  z = 1,
}) {
  return {
    id,
    kind,
    layout: "primary",
    payload: {
      event: {
        id: `${id}:event`,
        session_key: "session:dock",
        round_id: "round:dock",
        agent_id: "agent-stage",
        message_id: "msg-dock",
        kind: "unknown",
        surface: "fallback",
        phase,
        updated_at: 1,
      },
      snapshot: null,
    },
    phase,
    target,
    title,
    z,
  };
}

function verify_stage_window_drag_model() {
  const clamped = normalizeStageWindowDragOffset({ x: 9999, y: -9999 });
  assert(clamped.x === 520, `Window drag x should be clamped, got ${clamped.x}`);
  assert(clamped.y === -260, `Window drag y should be clamped, got ${clamped.y}`);
  assert(!isMeaningfulStageWindowDrag({ x: 1, y: -1 }), "Sub-pixel window drag should not leave maximized mode");
  assert(isMeaningfulStageWindowDrag({ x: 12, y: 0 }), "Visible window drag should be meaningful");
  const invalid = normalizeStageWindowDragOffset({ x: Number.NaN, y: Infinity });
  assert(invalid.x === 0 && invalid.y === 0, `Invalid drag offsets should reset to zero, got ${invalid.x}, ${invalid.y}`);
  const resized = normalizeStageWindowResizeSize({ height: 9999, width: 10 });
  assert(resized.height === 1000 && resized.width === 320, `Window resize should clamp to usable bounds, got ${resized.width}x${resized.height}`);
}

function verify_stage_window_launch_model() {
  const active = buildStageWindowLaunchState({
    index: 2,
    is_active: true,
    window: mock_stage_window({ id: "terminal", kind: "terminal", phase: "focused" }),
  });
  assert(active.delay_ms === 0, `Active window should launch immediately, got ${active.delay_ms}`);
  assert(active.origin === "dock", `Active app windows should launch from Dock, got ${active.origin}`);

  const browser = buildStageWindowLaunchState({
    index: 1,
    is_active: false,
    window: mock_stage_window({ id: "browser", kind: "browser", phase: "background" }),
  });
  assert(browser.origin === "dock", `App windows should launch from Dock, got ${browser.origin}`);
  assert(browser.delay_ms === 250, `Background app window should stagger after active window, got ${browser.delay_ms}`);

  const finder = buildStageWindowLaunchState({
    index: 3,
    is_active: true,
    window: { ...mock_stage_window({ id: "finder", kind: "finder", phase: "focused" }), layout: "secondary" },
  });
  assert(finder.origin === "desktop", `Finder should launch from desktop context, got ${finder.origin}`);
  assert(finder.delay_ms === 0, `Focused Finder should launch immediately from desktop context, got ${finder.delay_ms}`);
}

function verify_stage_window_position_model() {
  const background_code = mock_stage_window({ id: "code", kind: "code_editor", phase: "background" });
  const code_position = positionForWindow(
    background_code,
    "running",
  );
  assert(code_position.includes("left-[9%]"), `Background Code should remain in the natural desktop stack, got ${code_position}`);
  assert(code_position.includes("w-[80%]"), `Background Code should preserve a usable real window surface, got ${code_position}`);
  assert(isStageBackgroundWindow(background_code), "Running background windows should remain in the desktop window stack");

  const browser_position = positionForWindow(
    mock_stage_window({ id: "browser", kind: "browser", phase: "background" }),
    "running",
    2,
  );
  assert(browser_position.includes("top-[7%]"), `Background Navi should use a distinct cascading window offset, got ${browser_position}`);

  const focused_browser_position = positionForWindow(
    mock_stage_window({ id: "browser-focused", kind: "browser", phase: "focused" }),
    "running",
  );
  assert(focused_browser_position.includes("w-[86%]"), `Focused Navi should read as the primary desktop app, got ${focused_browser_position}`);
  assert(focused_browser_position.includes("h-[70%]"), `Focused Navi should leave enough height for real pages above the Dock, got ${focused_browser_position}`);

  const terminal_position = positionForWindow(
    { ...mock_stage_window({ id: "terminal", kind: "terminal", phase: "focused" }), layout: "terminal" },
    "running",
  );
  assert(terminal_position.includes("w-[84%]"), `Focused terminal should use the available desktop width, got ${terminal_position}`);
  assert(terminal_position.includes("h-[64%]"), `Focused terminal should keep output readable above the Dock, got ${terminal_position}`);

  const focused_code_position = positionForWindow(
    mock_stage_window({ id: "code-focused", kind: "code_editor", phase: "focused" }),
    "running",
  );
  assert(focused_code_position.includes("w-[61%]"), `Focused Code should leave the Activity Center rail visible, got ${focused_code_position}`);
  assert(focused_code_position.includes("h-[61%]"), `Focused Code should keep a visible desktop gap above the Dock, got ${focused_code_position}`);

  const permission_position = positionForWindow(
    mock_stage_window({ id: "permission", kind: "permission_wait", phase: "focused" }),
    "running",
  );
  assert(permission_position.includes("h-[62%]"), `Permission window should have enough System Settings height, got ${permission_position}`);
  assert(permission_position.includes("w-[56%]"), `Permission window should read like a System Settings window, got ${permission_position}`);

  const handoff_position = positionForWindow(
    mock_stage_window({ id: "handoff", kind: "handoff", phase: "focused" }),
    "completed",
  );
  assert(handoff_position.includes("h-[61%]"), `Focused handoff window should not collide with the Dock, got ${handoff_position}`);

  assert(isStageBackgroundWindow(background_code), "Completed review should keep prior windows in the desktop stack");
  assert(isStageBackgroundWindow(
    mock_stage_window({ id: "opening-code", kind: "code_editor", phase: "opening" }),
  ), "Opening non-focused windows should join the background stack without becoming fake thumbnails");
}

function verify_stage_live_strip_tracks_current_tool() {
  const read_event = {
    id: "tool-read",
    session_key: "session:stage",
    round_id: "round-strip",
    agent_id: "agent-stage",
    tool_use_id: "tool-read",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "workspace",
    phase: "done",
    title: "读取文件",
    target: "app.ts",
    updated_at: 1,
  };
  const bash_event = {
    ...read_event,
    id: "tool-bash",
    tool_use_id: "tool-bash",
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    phase: "running",
    title: "运行命令",
    target: "npm test",
    updated_at: 2,
  };
  const strip = buildStageLiveStripState({
    active_event: bash_event,
    active_window: mock_stage_window({
      id: "terminal",
      kind: "terminal",
      phase: "focused",
      target: "npm test",
      title: "npm test",
    }),
    events: [read_event, bash_event],
  });
  assert(strip.step_label === "第 2 步", `Live strip should track event sequence, got ${strip.step_label}`);
  assert(strip.app_label === "终端", `Live strip should expose active Mac app, got ${strip.app_label}`);
  assert(strip.detail === "运行 · npm test", `Live strip should describe current tool target, got ${strip.detail}`);
  assert(strip.tone === "active", `Running live strip should be active, got ${strip.tone}`);

  const done_strip = buildStageLiveStripState({
    active_event: { ...bash_event, phase: "done" },
    active_window: null,
    events: [read_event, bash_event],
  });
  assert(done_strip.tone === "done", `Done live strip should settle, got ${done_strip.tone}`);
}

function verify_operation_stage_key_is_session_scoped() {
  const identity = {
    chat_type: "group",
    conversation_id: "conversation-1",
    room_session_id: null,
    session_key: "agent:agent-a:websocket:group:runtime-session-a",
  };
  assert(
    buildOperationStageKey(identity) === "session:agent:agent-a:websocket:group:runtime-session-a",
    `Stage key should prefer runtime session over conversation, got ${buildOperationStageKey(identity)}`,
  );
  assert(
    buildOperationStageKey({ ...identity, room_session_id: "room-session-a" }) === "room-session:room-session-a",
    "Stage key should keep explicit room_session_id as the strongest isolation key",
  );
  assert(
    buildOperationStageKey({ ...identity, session_key: null }) === "room-conversation:conversation-1",
    "Stage key should fall back to conversation only when runtime session identity is missing",
  );
}

function verify_stage_experience_state_machine(now) {
  const base_event = {
    id: "event-state",
    session_key: "session:stage",
    round_id: "round-state",
    agent_id: "agent-stage",
    kind: "round_summary",
    surface: "summary",
    title: "State",
    updated_at: now,
  };
  assert(
    deriveOperationStageExperiencePhase(null, null) === "idle",
    "missing active event should keep stage in idle phase",
  );
  assert(
    deriveOperationStageExperiencePhase({ ...base_event, phase: "queued" }, null) === "awakening",
    "queued event should enter awakening phase",
  );
  assert(
    deriveOperationStageExperiencePhase({ ...base_event, phase: "running" }, null) === "running",
    "running event should enter running phase",
  );
  assert(
    deriveOperationStageExperiencePhase({ ...base_event, phase: "waiting" }, null) === "running",
    "waiting event should remain in running phase with a checkpoint surface",
  );
  assert(
    deriveOperationStageExperiencePhase({ ...base_event, phase: "error" }, null) === "settling",
    "error event should settle into review phase",
  );

  const single_done_event = { ...base_event, phase: "done" };
  assert(
    deriveOperationStageExperiencePhase(single_done_event, {
      key: "session:stage",
      session_key: "session:stage",
      active_event: single_done_event,
      events: [single_done_event],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    }) === "settling",
    "single completed event should settle before full completion",
  );

  const previous_tool_event = {
    ...base_event,
    id: "event-state-tool",
    kind: "workspace_read",
    surface: "workspace",
    phase: "done",
    title: "Read",
    target: "gomoku.html",
    updated_at: now - 100,
  };
  assert(
    deriveOperationStageExperiencePhase(single_done_event, {
      key: "session:stage",
      session_key: "session:stage",
      active_event: single_done_event,
      events: [previous_tool_event, single_done_event],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    }) === "completed",
    "multi-step completed round should enter completed phase",
  );
}

function verify_live_episode_narrates_running_round(now) {
  const base = {
    session_key: "session:stage-live",
    round_id: "round-live",
    agent_id: "agent-stage",
    updated_at: now,
  };
  const read_event = {
    ...base,
    id: "live-read",
    tool_use_id: "tool-read",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "editor",
    phase: "done",
    title: "Read index",
    target: "index.html",
    updated_at: now - 300,
  };
  const write_event = {
    ...base,
    id: "live-write",
    tool_use_id: "tool-write",
    tool_name: "Write",
    kind: "workspace_edit",
    surface: "editor",
    phase: "done",
    title: "Write gomoku",
    target: "gomoku.html",
    updated_at: now - 200,
  };
  const terminal_event = {
    ...base,
    id: "live-bash",
    tool_use_id: "tool-bash",
    tool_name: "Bash",
    kind: "command_run",
    surface: "terminal",
    phase: "running",
    title: "Run open",
    target: "open gomoku.html",
    updated_at: now - 100,
  };
  const episode = buildOperationLiveEpisode(
    terminal_event,
    [read_event, write_event, terminal_event],
    {
      key: "session:stage-live",
      session_key: "session:stage-live",
      active_event: terminal_event,
      events: [read_event, write_event, terminal_event],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  );

  assert(episode.status_label === "现场执行", `running tool should be narrated as live operation, got ${episode.status_label}`);
  assert(episode.progress_label === "3/3", `live episode should expose current event position, got ${episode.progress_label}`);
  assert(episode.settled_count === 2, `live episode should count settled predecessors, got ${episode.settled_count}`);
  assert(episode.previous_label.includes("Write"), `live episode should point to previous settled tool, got ${episode.previous_label}`);
  assert(episode.next_label.includes("命令退出"), `terminal live episode should wait for command exit, got ${episode.next_label}`);
  assert(episode.checkpoints.some((item) => item.label === "当前" && item.value === "执行"), "live episode should mark current step as executing");
}

function verify_api_retry_runtime_projection(now) {
  const messages = [{
    role: "assistant",
    message_id: "system_api_retry_round-retry",
    session_key: "session:retry",
    agent_id: "agent-stage",
    round_id: "round-retry",
    timestamp: now - 1000,
    is_complete: false,
    content: [{
      type: "system_event",
      subtype: "api_retry",
      label: "API 正在重试",
      content: "模型请求暂未成功，正在重试",
      tone: "warning",
      icon: "retry",
      source_message_id: "system_api_retry_round-retry",
      timestamp: now - 100,
    }],
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:retry",
    session_key: "session:retry",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: ["round-retry"],
    workspace_events: [],
  });
  const active_event = snapshot.active_event;
  assert(active_event?.title === "API 正在重试", `api retry should become explicit stage title, got ${active_event?.title}`);
  assert(active_event?.target === "模型请求暂未成功，正在重试", `api retry should preserve retry detail, got ${active_event?.target}`);
  assert(active_event?.evidence?.some((item) => item.label === "api_retry"), "api retry event should carry retry evidence");
  const episode = buildOperationLiveEpisode(active_event, snapshot.events, snapshot);
  assert(episode.status_label === "API 重试中", `api retry should narrate as retrying, got ${episode.status_label}`);
  assert(episode.next_label.includes("模型响应"), `api retry should wait for model response, got ${episode.next_label}`);
}

function verify_active_event_stays_with_latest_round(now) {
  const messages = [{
    role: "assistant",
    message_id: "msg-old-running",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-old",
    timestamp: now - 3000,
    is_complete: false,
    content: [{
      type: "tool_use",
      id: "tool-old-bash",
      name: "Bash",
      input: {
        command: "sleep 999",
      },
    }],
  }, {
    role: "assistant",
    message_id: "msg-new-summary",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-new",
    timestamp: now - 1000,
    is_complete: true,
    content: [{
      type: "text",
      text: "new round done",
    }],
    result_summary: {
      subtype: "success",
      duration_ms: 500,
      duration_api_ms: 400,
      num_turns: 1,
      result: "new round done",
      is_error: false,
      timestamp: now - 900,
    },
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events: [],
  });

  assert(snapshot.events.some((event) => event.round_id === "round-old" && event.phase === "running"), "fixture should include an older running event");
  assert(snapshot.active_event?.round_id === "round-new", `active event should follow latest round, got ${snapshot.active_event?.round_id}`);
  assert(snapshot.active_event?.kind === "round_summary", `latest completed round should focus summary, got ${snapshot.active_event?.kind}`);
}

function verify_error_summary_settles_live_handoff(now) {
  const live_handoff = {
    id: "live-round:round-error",
    session_key: "session:error",
    round_id: "round-error",
    agent_id: "agent-stage",
    message_id: "system_api_retry_round-error",
    kind: "unknown",
    surface: "conversation",
    phase: "running",
    title: "API 正在重试",
    target: "模型请求暂未成功，正在重试",
    evidence: [{ type: "status", label: "api_retry", value: "API 正在重试" }],
    updated_at: now - 1000,
  };
  const error_summary = {
    id: "summary-error",
    session_key: "session:error",
    round_id: "round-error",
    agent_id: "agent-stage",
    kind: "round_summary",
    surface: "summary",
    phase: "error",
    title: "本轮执行异常",
    target: "1 turns",
    summary: "Failed to authenticate",
    evidence: [{ type: "error", label: "error", value: "Failed to authenticate" }],
    updated_at: now,
    ended_at: now,
  };
  const current = {
    key: "session:error",
    session_key: "session:error",
    active_event: live_handoff,
    events: [live_handoff],
    recent_evidence: [],
    workspace_events: [],
    updated_at: now - 900,
  };
  const next = {
    key: "session:error",
    session_key: "session:error",
    active_event: error_summary,
    events: [error_summary],
    recent_evidence: error_summary.evidence,
    workspace_events: [],
    updated_at: now,
  };

  const merged = mergeOperationStageSnapshotsForRestore(current, next);
  const settled_handoff = merged.events.find((event) => event.id === live_handoff.id);
  assert(merged.active_event?.id === error_summary.id, "error summary should remain active after merge");
  assert(settled_handoff?.phase === "error", `stale live handoff should be settled as error, got ${settled_handoff?.phase}`);
  const brief = buildOperationContinuationBrief(merged.active_event, merged.events, merged);
  assert(brief.checkpoints.every((item) => !String(item.value).includes("个活动")), "error completion brief should not report active running windows");
}

function verify_stage_restore_merge_preserves_round_context(now) {
  const restored_read = {
    id: "restored-read",
    session_key: "session:restore",
    round_id: "round-restore",
    agent_id: "agent-stage",
    tool_use_id: "tool-read",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "editor",
    phase: "done",
    title: "Read",
    target: "index.html",
    updated_at: now - 400,
  };
  const restored_write = {
    id: "restored-write",
    session_key: "session:restore",
    round_id: "round-restore",
    agent_id: "agent-stage",
    tool_use_id: "tool-write",
    tool_name: "Write",
    kind: "workspace_edit",
    surface: "editor",
    phase: "done",
    title: "Write",
    target: "gomoku.html",
    updated_at: now - 300,
  };
  const projected_summary = {
    id: "projected-summary",
    session_key: "session:restore",
    round_id: "round-restore",
    agent_id: "agent-stage",
    kind: "round_summary",
    surface: "summary",
    phase: "done",
    title: "本轮执行收口",
    target: "1 turns",
    updated_at: now - 100,
  };
  const current = {
    key: "session:restore",
    session_key: "session:restore",
    active_event: restored_write,
    events: [restored_read, restored_write],
    recent_evidence: [{ type: "artifact", label: "gomoku", value: "gomoku.html" }],
    workspace_events: [{
      id: "workspace-gomoku",
      agent_id: "agent-stage",
      path: "gomoku.html",
      status: "updated",
      version: 1,
      source: "agent",
      session_key: "session:restore",
      tool_use_id: "tool-write",
      live_content: "<html />",
      updated_at: now - 250,
      event_type: "file_write_end",
    }],
    updated_at: now - 200,
  };
  const next = {
    key: "session:restore",
    session_key: "session:restore",
    active_event: projected_summary,
    events: [projected_summary],
    recent_evidence: [{ type: "status", label: "duration", value: "1s" }],
    workspace_events: [],
    updated_at: now,
  };

  const merged = mergeOperationStageSnapshotsForRestore(current, next);
  assert(merged.active_event?.id === "projected-summary", "restore merge should keep projected active event");
  assert(merged.events.some((event) => event.id === "restored-read"), "restore merge should preserve earlier read event from restored stage snapshot");
  assert(merged.events.some((event) => event.id === "restored-write"), "restore merge should preserve earlier write event from restored stage snapshot");
  assert(merged.events.at(-1)?.id === "projected-summary", "restore merge should keep projected summary at the end of the round");
  assert(merged.workspace_events.some((item) => item.path === "gomoku.html"), "restore merge should preserve workspace artifact for restored round");
  assert(merged.recent_evidence.some((item) => item.label === "duration"), "restore merge should include fresh projected evidence");
  assert(merged.recent_evidence.some((item) => item.label === "gomoku"), "restore merge should include restored artifact evidence");
}

function verify_workspace_live_stays_in_tool_round(now) {
  const messages = [{
    role: "assistant",
    message_id: "msg-summary",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-stage",
    timestamp: now - 1000,
    is_complete: true,
    content: [
      {
        type: "tool_use",
        id: "tool-write",
        name: "Write",
        input: {
          file_path: "gomoku.html",
          content: "<html />",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-write",
        content: "wrote gomoku.html",
        is_error: false,
      },
    ],
    result_summary: {
      subtype: "success",
      duration_ms: 1200,
      duration_api_ms: 900,
      num_turns: 1,
      result: "done",
      is_error: false,
      timestamp: now - 500,
    },
  }];
  const workspace_events = [{
    id: "workspace-late",
    agent_id: "agent-stage",
    path: "gomoku.html",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-write",
    event_type: "file_write_end",
    live_content: "<html />",
    diff_stats: {
      additions: 1,
      deletions: 0,
      changed_lines: 1,
    },
    updated_at: now,
  }, {
    id: "workspace-stale",
    agent_id: "agent-stage",
    path: "stale-session.md",
    status: "updated",
    version: 8,
    source: "agent",
    session_key: "session:old",
    tool_use_id: "tool-stale",
    event_type: "file_write_end",
    live_content: "old session content",
    updated_at: now - 200,
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events,
  });
  const workspace_event = snapshot.events.find((event) => event.id === "workspace:workspace-late");
  assert(workspace_event, "workspace live event should be projected");
  assert(workspace_event.tool_use_id === "tool-write", `workspace live event should preserve tool identity, got ${workspace_event.tool_use_id}`);
  assert(!snapshot.workspace_events.some((item) => item.path === "stale-session.md"), "workspace events from another session should not enter stage snapshot");
  assert(!snapshot.events.some((event) => event.target === "stale-session.md"), "workspace events from another session should not be projected as current stage events");
  assert(workspace_event.round_id === "round-stage", `workspace live event should stay in tool round, got ${workspace_event.round_id}`);
  assert(snapshot.active_event?.kind === "round_summary", `completed stage should focus round summary, got ${snapshot.active_event?.kind}`);
  const desktop = planOperationDesktop({
    event: snapshot.active_event,
    snapshot,
  });
  assert(desktop.active_window_id?.includes(":browser"), `completed stage should keep the artifact browser focused, got ${desktop.active_window_id}`);
  assert(!desktop.windows.some((window) => window.kind === "handoff"), "completed stage with real app windows should not render a handoff app window");
  const continuation_brief = buildOperationContinuationBrief(snapshot.active_event, snapshot.events, snapshot);
  assert(continuation_brief.status_label === "可继续", `completed stage continuation brief should be ready, got ${continuation_brief.status_label}`);
  assert(continuation_brief.primary_artifact === "gomoku.html", `completed stage continuation brief should point to current artifact, got ${continuation_brief.primary_artifact}`);
  assert(continuation_brief.resume_prompt.includes("gomoku.html"), "completed stage continuation prompt should point to current artifact");
  const browser_window = desktop.windows.find((window) => window.kind === "browser");
  assert(browser_window?.phase === "focused", `html artifact should stay focused after handoff, got ${browser_window?.phase}`);
  const terminal_window = desktop.windows.find((window) => window.kind === "terminal");
  if (terminal_window) {
    assert(terminal_window.phase === "minimized", `completed terminal should return to Dock, got ${terminal_window.phase}`);
  }
  const code_window = desktop.windows.find((window) => window.kind === "code_editor");
  assert(code_window?.phase === "minimized", `completed source editor should return to Dock when Navi shows the artifact, got ${code_window?.phase}`);
  assert(!desktop.windows.some((window) => window.target === "stale-session.md"), "completed stage should not render stale workspace windows");
  const write_event = snapshot.events.find((event) => event.tool_use_id === "tool-write");
  assert(write_event, "write tool event should be projected");
  const write_window_id = resolveOperationEventWindowId(write_event, desktop.windows);
  assert(write_window_id?.includes(":document:gomoku.html"), `write event should focus gomoku document window, got ${write_window_id}`);
  const summary_window_id = resolveOperationEventWindowId(snapshot.active_event, desktop.windows);
  assert(summary_window_id?.includes(":browser"), `summary event should resolve to the focused app window, got ${summary_window_id}`);
}

function verify_multi_file_windows_keep_event_identity(now) {
  const messages = [{
    role: "assistant",
    message_id: "msg-multi-file",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-multi-file",
    timestamp: now - 1500,
    is_complete: true,
    content: [
      {
        type: "tool_use",
        id: "tool-html",
        name: "Write",
        input: {
          file_path: "gomoku.html",
          content: "<html><body>board</body></html>",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-html",
        content: "created gomoku.html",
        is_error: false,
      },
      {
        type: "tool_use",
        id: "tool-css",
        name: "Write",
        input: {
          file_path: "style.css",
          content: "body { margin: 0; }",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-css",
        content: "created style.css",
        is_error: false,
      },
    ],
    result_summary: {
      subtype: "success",
      duration_ms: 1500,
      duration_api_ms: 1200,
      num_turns: 1,
      result: "created app",
      is_error: false,
      timestamp: now - 100,
    },
  }];
  const workspace_events = [{
    id: "workspace-html",
    agent_id: "agent-stage",
    path: "gomoku.html",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-html",
    event_type: "file_write_end",
    live_content: "<html><body>board</body></html>",
    updated_at: now - 600,
  }, {
    id: "workspace-css",
    agent_id: "agent-stage",
    path: "style.css",
    status: "updated",
    version: 1,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-css",
    event_type: "file_write_end",
    live_content: "body { margin: 0; }",
    updated_at: now - 500,
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events,
  });
  const desktop = planOperationDesktop({
    event: snapshot.active_event,
    snapshot,
  });
  assert(desktop.windows.some((window) => window.id.includes(":document:gomoku.html")), "multi-file stage should keep gomoku document window");
  assert(desktop.windows.some((window) => window.id.includes(":document:style.css")), "multi-file stage should keep style document window");
  const html_event = snapshot.events.find((event) => event.tool_use_id === "tool-html");
  const css_event = snapshot.events.find((event) => event.tool_use_id === "tool-css");
  assert(html_event, "html write event should exist");
  assert(css_event, "css write event should exist");
  const html_window_id = resolveOperationEventWindowId(html_event, desktop.windows);
  const css_window_id = resolveOperationEventWindowId(css_event, desktop.windows);
  assert(html_window_id?.includes(":document:gomoku.html"), `html event should focus gomoku window, got ${html_window_id}`);
  assert(css_window_id?.includes(":document:style.css"), `css event should focus style window, got ${css_window_id}`);
  assert(
    resolveFilePreviewValue(html_event, null) === "<html><body>board</body></html>",
    "html write window should render file content before tool result text",
  );
  assert(
    resolveFilePreviewValue(css_event, null) === "body { margin: 0; }",
    "css write window should render file content before tool result text",
  );
  assert(
    resolveFilePreviewValue(html_event, "<html><body>live board</body></html>") === "<html><body>live board</body></html>",
    "workspace live content should override stale write input in Code window",
  );
  const active_css_desktop = planOperationDesktop({
    event: css_event,
    snapshot,
  });
  assert(active_css_desktop.active_window_id?.includes(":document:style.css"), `active workspace write should focus its document window, got ${active_css_desktop.active_window_id}`);
}

function verify_extensionless_workspace_file_opens_code_app(now) {
  const read_event = {
    id: "tool-read-makefile",
    session_key: "session:stage",
    round_id: "round-extensionless-file",
    agent_id: "agent-stage",
    tool_use_id: "tool-read",
    tool_name: "Read",
    kind: "workspace_read",
    surface: "workspace",
    phase: "done",
    title: "Read Makefile",
    target: "Makefile",
    result_preview: "test:\n\tpnpm test",
    updated_at: now,
  };
  const snapshot = {
    key: "session:stage",
    session_key: "session:stage",
    active_event: read_event,
    events: [read_event],
    recent_evidence: [],
    workspace_events: [],
    updated_at: now,
  };
  const desktop = planOperationDesktop({
    event: read_event,
    snapshot,
  });
  const document_window = desktop.windows.find((window) => window.target === "Makefile");
  assert(document_window, "extensionless workspace file should still open a document window");
  assert(document_window.kind === "code_editor", `extensionless workspace file should open in Code, got ${document_window.kind}`);
  assert(appSurfaceForWindowKind(document_window.kind) === "document", "extensionless workspace file should render as document content");
}

function verify_code_writer_preview_uses_real_content(now) {
  const base_event = {
    id: "tool-edit-app",
    session_key: "session:stage",
    round_id: "round-code-writer",
    agent_id: "agent-stage",
    message_id: "message-code-writer",
    kind: "workspace_edit",
    surface: "editor",
    phase: "done",
    title: "修改文件",
    target: "src/app.ts",
    updated_at: now,
  };
  const write_event = {
    ...base_event,
    id: "tool-write-app",
    tool_name: "Write",
    tool_use_id: "tool-write",
    input_preview: {
      file_path: "src/app.ts",
      content: "export const app = true;\n",
    },
  };
  assert(
    resolveFilePreviewValue(write_event, write_event.input_preview) === "export const app = true;\n",
    "Write preview should render the file content instead of the tool input JSON",
  );

  const edit_event = {
    ...base_event,
    tool_name: "Edit",
    tool_use_id: "tool-edit",
    input_preview: {
      file_path: "src/app.ts",
      old_string: "export const app = false;",
      new_string: "export const app = true;",
    },
  };
  const edit_preview = resolveFilePreviewValue(edit_event, edit_event.input_preview);
  assert(
    edit_preview === "- export const app = false;\n+ export const app = true;",
    `Edit preview should render a real old/new text hunk, got ${edit_preview}`,
  );

  const multi_edit_event = {
    ...base_event,
    id: "tool-multiedit-app",
    tool_name: "MultiEdit",
    tool_use_id: "tool-multiedit",
    input_preview: {
      file_path: "src/app.ts",
      edits: [
        { old_string: "const a = 1;", new_string: "const a = 2;" },
        { old_string: "const b = 1;", new_string: "const b = 2;" },
      ],
    },
  };
  const multi_edit_preview = resolveFilePreviewValue(multi_edit_event, multi_edit_event.input_preview);
  assert(
    typeof multi_edit_preview === "string" &&
      multi_edit_preview.includes("- const a = 1;") &&
      multi_edit_preview.includes("+ const b = 2;"),
    `MultiEdit preview should render real edit hunks, got ${multi_edit_preview}`,
  );

  const desktop = planOperationDesktop({
    event: edit_event,
    snapshot: {
      key: "session:stage",
      session_key: "session:stage",
      active_event: edit_event,
      events: [write_event, edit_event],
      recent_evidence: [],
      runtime_events: [],
      workspace_events: [
        {
          id: "workspace-app-old",
          agent_id: "agent-stage",
          path: "src/app.ts",
          status: "updated",
          version: 1,
          source: "agent",
          session_key: "session:stage",
          tool_use_id: "tool-write",
          event_type: "file_write_end",
          live_content: "export const app = false;",
          updated_at: now - 1000,
        },
        {
          id: "workspace-app-latest",
          agent_id: "agent-stage",
          path: "src/app.ts",
          status: "writing",
          version: 2,
          source: "agent",
          session_key: "session:stage",
          tool_use_id: "tool-edit",
          event_type: "file_write_delta",
          live_content: "export const app = true;",
          diff_stats: { additions: 1, deletions: 1, changed_lines: 1 },
          updated_at: now,
        },
      ],
      updated_at: now,
    },
  });
  const document_window = desktop.windows.find((window) => window.target === "src/app.ts");
  assert(document_window, "Code writer should open a document window for the edited file");
  assert(document_window.payload.preview === "export const app = true;", `Code writer should render latest live content, got ${document_window.payload.preview}`);
  assert(document_window.payload.diff_stats?.additions === 1, "Code writer should keep latest workspace diff stats");
}

function verify_code_editor_session_view() {
  const ts_view = buildCodeEditorSessionView({
    diff_stats: { additions: 3, deletions: 1 },
    lines: ["export const answer = 42;"],
    title: "stage-preview.tsx",
  });
  assert(ts_view.tab_title === "stage-preview.tsx", `Code tab should keep file title, got ${ts_view.tab_title}`);
  assert(ts_view.extension_label === "TSX", `Code extension badge should use uppercase extension, got ${ts_view.extension_label}`);
  assert(ts_view.language_label === "TypeScript React", `Code language should map tsx, got ${ts_view.language_label}`);
  assert(ts_view.status_label.includes("+3 -1"), `Code status should include diff stats, got ${ts_view.status_label}`);
  assert(ts_view.cursor_label === "Ln 1, Col 1", `Code cursor should track line count, got ${ts_view.cursor_label}`);

  const text_view = buildCodeEditorSessionView({
    lines: [],
    title: "Makefile",
  });
  assert(!text_view.is_code, "Extensionless files should use plain text editor semantics");
  assert(text_view.status_label.includes("Plain Text"), `Extensionless status should be plain text, got ${text_view.status_label}`);
  assert(text_view.cursor_label === "Ln 1, Col 1", `Empty editor should still expose first line cursor, got ${text_view.cursor_label}`);
}

function verify_terminal_result_envelope(now) {
  const messages = [{
    role: "assistant",
    message_id: "msg-terminal",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-terminal",
    timestamp: now - 1000,
    is_complete: true,
    content: [
      {
        type: "tool_use",
        id: "tool-bash",
        name: "Bash",
        input: {
          command: "printf \"1\\n2\\n\"",
        },
      },
      {
        type: "task_progress",
        task_id: "tool-bash",
        tool_use_id: "tool-bash",
        last_tool_name: "Bash",
        description: "Bash 正在执行",
        terminal_output: {
          kind: "snapshot",
          stream: "combined",
          text: "partial\n",
        },
        usage: {
          duration_ms: 2350,
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-bash",
        content: "1\n2\n",
        is_error: false,
        error_code: null,
      },
    ],
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events: [],
  });
  const terminal_event = snapshot.events.find((event) => event.tool_use_id === "tool-bash");
  assert(terminal_event, "terminal tool event should be projected");
  assert(terminal_event.kind === "command_run", `terminal kind should be command_run, got ${terminal_event.kind}`);
  assert(terminal_event.surface === "terminal", `terminal surface should be terminal, got ${terminal_event.surface}`);
  assert(terminal_event.result_preview?.content === "1\n2\n", "terminal output content should be preserved");
  assert(terminal_event.result_preview?.is_error === false, "terminal success state should be preserved");
  assert(terminal_event.duration_ms === 2350, `terminal should preserve SDK progress duration, got ${terminal_event.duration_ms}`);
  assert(!snapshot.events.some((event) => event.kind === "task_progress"), "Bash tool_progress must not create an Activity Monitor event");
}

function verify_nxs_terminal_progress_and_claude_fallback(now) {
  const baseMessage = {
    role: "assistant",
    message_id: "msg-terminal-live",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-terminal-live",
    timestamp: now - 1000,
    is_complete: false,
  };
  const toolUse = {
    type: "tool_use",
    id: "tool-bash-live",
    name: "Bash",
    input: {
      command: "printf \"first\\nsecond\\n\"",
    },
  };
  const nxsMessages = [{
    ...baseMessage,
    content: [
      toolUse,
      {
        type: "task_progress",
        task_id: "tool-bash-live",
        tool_use_id: "tool-bash-live",
        last_tool_name: "Bash",
        description: "Bash 正在执行",
        terminal_output: {
          kind: "snapshot",
          stream: "combined",
          text: "first\nsecond\n",
          total_bytes: 13,
          total_lines: 2,
        },
        usage: {
          duration_ms: 2100,
        },
      },
    ],
  }];
  const nxsSnapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages: nxsMessages,
    pending_permissions: [],
    live_round_ids: ["round-terminal-live"],
    workspace_events: [],
  });
  const nxsEvent = nxsSnapshot.events.find((event) => event.tool_use_id === "tool-bash-live");
  assert(nxsEvent?.phase === "running", `nxs terminal progress should stay running, got ${nxsEvent?.phase}`);
  assert(nxsEvent?.result_preview?.content === "first\nsecond\n", "nxs terminal should expose the latest cumulative output snapshot");
  const [nxsEntry] = buildTerminalSession({ event: nxsEvent, relatedEvents: [] }).entries;
  assert(nxsEntry.result.rows.map((row) => row.text).join("\n") === "first\nsecond", "nxs terminal should render live output rows");

  const runtimeDelta = nxsSnapshot.runtime_events.find((event) => (
    event.event_type === "tool_delta" && event.tool_use_id === "tool-bash-live"
  ));
  assert(runtimeDelta?.delta?.terminal_output?.text === "first\nsecond\n", "nxs runtime delta should carry terminal output");
  const runtimeOperation = operationEventFromRuntimeEvent(runtimeDelta);
  assert(runtimeOperation.result_preview?.content === "first\nsecond\n", "runtime desktop projection should preserve nxs terminal output");

  const claudeMessages = [{
    ...baseMessage,
    message_id: "msg-terminal-claude",
    content: [
      toolUse,
      {
        type: "task_progress",
        task_id: "tool-bash-live",
        tool_use_id: "tool-bash-live",
        last_tool_name: "Bash",
        description: "Bash 正在执行",
        usage: {
          duration_ms: 2100,
        },
      },
    ],
  }];
  const claudeSnapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages: claudeMessages,
    pending_permissions: [],
    live_round_ids: ["round-terminal-live"],
    workspace_events: [],
  });
  const claudeEvent = claudeSnapshot.events.find((event) => event.tool_use_id === "tool-bash-live");
  assert(claudeEvent?.phase === "running", `claude terminal progress should stay running, got ${claudeEvent?.phase}`);
  assert(claudeEvent?.result_preview == null, "claude progress must not fabricate terminal output before tool_result");
  const [claudeEntry] = buildTerminalSession({ event: claudeEvent, relatedEvents: [] }).entries;
  assert(claudeEntry.result.rows.length === 0, "claude terminal should wait without output rows");
  assert(claudeEntry.statusLabel === "运行中", `claude terminal should show running status, got ${claudeEntry.statusLabel}`);
}

function verify_terminal_entries_render_real_command_result(now) {
  const success_event = {
    id: "terminal-success",
    session_key: "session:stage",
    round_id: "round-terminal",
    agent_id: "agent-stage",
    message_id: "msg-terminal",
    kind: "command_run",
    surface: "terminal",
    phase: "done",
    tool_name: "Bash",
    target: "printf \"1\\n2\\n\"",
    input_preview: {
      command: "printf \"1\\n2\\n\"",
      cwd: "/Users/berhand/.nexus/workspace/Miles",
    },
    result_preview: {
      content: {
        stdout: "1\n2\n",
        exit_code: 0,
      },
      is_error: false,
    },
    duration_ms: 2350,
    updated_at: now,
  };
  const error_event = {
    ...success_event,
    id: "terminal-error",
    phase: "error",
    target: "cat missing.txt",
    input_preview: {
      command: "cat missing.txt",
      cwd: "/Users/berhand/.nexus/workspace/Miles",
    },
    result_preview: {
      content: "cat: missing.txt: No such file or directory\n",
      is_error: true,
      exit_code: 1,
    },
  };
  const background_event = {
    ...success_event,
    id: "terminal-background",
    phase: "done",
    target: "pnpm dev",
    tool_use_id: "tool-background",
    input_preview: {
      command: "pnpm dev",
    },
    result_preview: {
      content: {
        task_id: "shell-42",
      },
      is_error: false,
    },
  };
  const stop_event = {
    ...success_event,
    id: "terminal-stop",
    kind: "command_stop",
    tool_name: "KillShell",
    phase: "done",
    title: "终止命令",
    target: "shell-42",
    input_preview: {
      shell_id: "shell-42",
    },
    result_preview: {
      content: {
        message: "Successfully stopped task: shell-42",
        task_id: "shell-42",
      },
      is_error: false,
    },
    duration_ms: 130,
    updated_at: now + 1,
  };

  const [success_entry] = buildTerminalSession({
    event: success_event,
    relatedEvents: [],
  }).entries;
  const permission_completion_event = {
    ...success_event,
    id: "permission:terminal-success",
    kind: "human_gate",
    surface: "conversation",
    phase: "done",
    tool_use_id: null,
    permission_request_id: "permission-terminal-success",
    result_preview: null,
    updated_at: now + 1,
  };
  const success_with_permission = buildTerminalSession({
    event: permission_completion_event,
    relatedEvents: [success_event, permission_completion_event],
  });
  const [stopped_entry] = buildTerminalSession({
    event: stop_event,
    relatedEvents: [background_event, stop_event],
  }).entries;
  const background_session = buildTerminalSession({
    event: background_event,
    relatedEvents: [background_event],
  });
  const [error_entry] = buildTerminalSession({
    event: error_event,
    relatedEvents: [],
  }).entries;
  const [unknown_entry] = buildTerminalSession({
    event: {
      ...success_event,
      id: "terminal-unknown-status",
      input_preview: { command: "echo ok" },
      result_preview: { content: "ok", is_error: false },
      duration_ms: null,
    },
    relatedEvents: [],
  }).entries;

  assert(success_entry.command === "printf \"1\\n2\\n\"", `terminal entry should preserve command, got ${success_entry.command}`);
  assert(success_entry.result.stdout.join("\n") === "1\n2", `terminal success content should preserve stdout, got ${success_entry.result.stdout.join("\\n")}`);
  assert(success_entry.result.stderr.length === 0, `terminal success should not populate stderr, got ${success_entry.result.stderr.length}`);
  assert(success_entry.statusLabel === "退出 0", `terminal success should show explicit exit 0, got ${success_entry.statusLabel}`);
  assert(success_entry.statusTone === "success", `terminal success should use success tone, got ${success_entry.statusTone}`);
  assert(success_entry.durationLabel === "2.4s", `terminal should use SDK duration, got ${success_entry.durationLabel}`);
  assert(success_with_permission.entries.length === 1, `completed Bash permission events must not duplicate the command, got ${success_with_permission.entries.length}`);
  assert(error_entry.result.stderr.some((row) => row.includes("missing.txt")), "terminal error transcript should keep stderr rows");
  assert(error_entry.statusLabel === "退出 1", `terminal error should show explicit exit 1, got ${error_entry.statusLabel}`);
  assert(error_entry.statusTone === "error", `terminal error should use error tone, got ${error_entry.statusTone}`);
  assert(unknown_entry.statusLabel === "已完成", `missing exit code should be omitted, got ${unknown_entry.statusLabel}`);
  assert(unknown_entry.durationLabel === null, `missing duration should be omitted, got ${unknown_entry.durationLabel}`);
  assert(unknown_entry.cwdLabel === null, `missing cwd must stay unknown, got ${unknown_entry.cwdLabel}`);
  assert(background_session.hasActiveProcess, "completed Bash startup with a task id should keep the background process active");
  assert(background_session.entries[0].statusLabel === "后台运行中", `background Bash should stay active, got ${background_session.entries[0].statusLabel}`);
  assert(background_session.entries[0].durationLabel === "启动 2.4s", `background Bash should label startup duration, got ${background_session.entries[0].durationLabel}`);
  assert(stopped_entry.command === "pnpm dev", `KillShell should keep the Bash command, got ${stopped_entry.command}`);
  assert(stopped_entry.statusLabel === "已终止", `KillShell should update the Bash process state, got ${stopped_entry.statusLabel}`);
  assert(stopped_entry.controls.length === 1, `KillShell should attach one control event, got ${stopped_entry.controls.length}`);
  assert(stopped_entry.controls[0].targetLabel === "shell-42", `KillShell should keep its real target, got ${stopped_entry.controls[0].targetLabel}`);
  assert(stopped_entry.controls[0].durationLabel === "130ms", `KillShell should keep its own duration, got ${stopped_entry.controls[0].durationLabel}`);
  assert(stopped_entry.controls[0].resultRows.some((row) => row.text.includes("Successfully stopped task")), "KillShell should only render its real result");
  assert(!JSON.stringify(stopped_entry).includes("kill-shell shell-42"), "KillShell must not fabricate a shell command");

  const cross_round_background = {
    ...background_event,
    id: "terminal-background-prior-round",
    round_id: "round-terminal-background",
    updated_at: now - 100,
  };
  const cross_round_stop = {
    ...stop_event,
    id: "terminal-stop-next-round",
    round_id: "round-terminal-stop",
    updated_at: now,
  };
  const cross_round_desktop = planOperationDesktop({
    event: cross_round_stop,
    snapshot: {
      key: "terminal-cross-round",
      session_key: "session:terminal-cross-round",
      active_event: cross_round_stop,
      events: [cross_round_background, cross_round_stop],
      runtime_events: [],
      recent_evidence: [],
      workspace_events: [],
      updated_at: now,
    },
  });
  const cross_round_terminal = cross_round_desktop.windows.find((window) => window.kind === "terminal");
  assert(cross_round_terminal, "cross-round KillShell should still open Terminal");
  const cross_round_session = buildTerminalSession({
    event: cross_round_stop,
    relatedEvents: cross_round_terminal.payload.related_events ?? [],
  });
  assert(cross_round_session.entries.length === 1, `cross-round KillShell should restore one Bash session, got ${cross_round_session.entries.length}`);
  assert(cross_round_session.entries[0].command === "pnpm dev", "cross-round KillShell should restore the exact prior Bash command");
  assert(cross_round_session.entries[0].statusLabel === "已终止", "cross-round KillShell should terminate the restored Bash session");
  assert(cross_round_session.entries[0].controls.length === 1, "cross-round KillShell should attach to the restored Bash session");

  const plainResult = parseTerminalResult({ content: "plain tool output", is_error: false });
  assert(plainResult.output[0] === "plain tool output", "plain tool content should stay generic output instead of fake stdout");
}

function verify_browser_fallback_builds_search_results(now) {
  const event = {
    id: "web-search",
    session_key: "session:stage",
    round_id: "round-web",
    agent_id: "agent-stage",
    message_id: "msg-web",
    kind: "web_research",
    surface: "web",
    phase: "done",
    tool_name: "web_search",
    target: "nexus stage mac desktop",
    summary: "Search completed",
    updated_at: now,
  };

  const items = buildBrowserResultItems({
    event,
    query: "nexus stage mac desktop",
    lines: [
      "[",
      "https://example.com/stage",
      "\"https://developer.apple.com/design/human-interface-guidelines/windows\",",
      "[Nexus Desktop](https://nexus.example.com/desktop) Window design notes",
      "Local summary without a URL",
      "]",
    ],
  });

  assert(items.length === 3, `browser fallback should group plain summary lines under the preceding URL, got ${items.length}`);
  assert(items[0].url === "https://example.com/stage", `plain URL result should preserve URL, got ${items[0].url}`);
  assert(items[0].kind === "link", `plain URL result should be rendered as a link, got ${items[0].kind}`);
  assert(items[0].title.includes("example.com"), `plain URL result should derive readable title, got ${items[0].title}`);
  assert(items[1].url === "https://developer.apple.com/design/human-interface-guidelines/windows", `quoted URL result should be cleaned, got ${items[1].url}`);
  assert(items[2].title === "Nexus Desktop", `markdown link result should preserve title, got ${items[2].title}`);
  assert(items[2].url === "https://nexus.example.com/desktop", `markdown link result should preserve URL, got ${items[2].url}`);
  assert(items[2].snippet.includes("Local summary without a URL"), `plain text should extend the preceding result snippet, got ${items[2].snippet}`);
}

function verify_browser_reader_highlights_tool_hits() {
  const paragraphs = buildBrowserReaderParagraphs({
    fallback: "fallback",
    lines: [
      "\"Pomodoro timers alternate focus intervals and short breaks.\",",
      "\"Users expect start, pause, reset, and session counters.\"",
    ],
    markers: ["Pomodoro timers"],
    preview: null,
  });
  assert(paragraphs.length === 2, `Reader should keep tool-returned paragraphs, got ${paragraphs.length}`);
  assert(paragraphs[0].highlighted, "Reader should highlight the paragraph matching the WebFetch intent");
  assert(!paragraphs[1].highlighted, "Reader should not mark unrelated fetched paragraphs as intent hits");
  assert(!paragraphs[0].text.includes("\"") && !paragraphs[0].text.endsWith(","), `Reader should strip JSON string noise, got ${paragraphs[0].text}`);

  const fallback = buildBrowserReaderParagraphs({
    fallback: "页面内容已抓取",
    lines: [],
    markers: [],
    preview: null,
  });
  assert(fallback[0].text === "页面内容已抓取", `Reader should show fallback when no body is available, got ${fallback[0].text}`);

  const noisy = buildBrowserReaderParagraphs({
    fallback: "抓取失败",
    lines: [
      "content: \"<html><body><script>window.onload=setTimeout('lw(12)', 200); var q=[0x95,0x40,0xcc,0xcb,0xc0,0xc5,0x59]</script>\"",
      "error_code: null",
      "is_error: true",
    ],
    markers: [],
    preview: null,
  });
  assert(noisy.length === 1 && noisy[0].text === "抓取失败", `Reader should hide html/error envelope noise, got ${noisy.map((item) => item.text).join(" | ")}`);
}

function verify_browser_session_view(now) {
  const base_event = {
    id: "browser-session",
    session_key: "session:stage",
    round_id: "round-browser-session",
    agent_id: "agent-stage",
    message_id: "msg-browser-session",
    kind: "web_research",
    surface: "web",
    phase: "done",
    tool_name: "web_fetch",
    target: "gomoku.html",
    updated_at: now,
  };
  const srcdoc_view = buildBrowserSessionView({
    event: base_event,
    preview: "<html><head><title>Gomoku Board</title></head><body>board</body></html>",
    query: "gomoku.html",
    target: "gomoku.html",
  });
  assert(srcdoc_view.srcdoc?.includes("board"), "Browser session should preserve inline html preview");
  assert(srcdoc_view.srcdoc?.includes('<base href="/nexus/v1/agents/agent-stage/workspace/site/">'), "Workspace srcdoc should resolve sibling assets through the site route");
  assert(srcdoc_view.iframe_url === "/nexus/v1/agents/agent-stage/workspace/site/gomoku.html", `Inline html should retain its real workspace URL, got ${srcdoc_view.iframe_url}`);
  assert(srcdoc_view.display_url === srcdoc_view.iframe_url, `Inline html should show its workspace URL, got ${srcdoc_view.display_url}`);
  assert(srcdoc_view.page_kind === "workspace", `Inline html should be workspace page kind, got ${srcdoc_view.page_kind}`);
  assert(srcdoc_view.source_label === "工作区", `Inline html source should be workspace, got ${srcdoc_view.source_label}`);
  assert(srcdoc_view.tab_title === "Gomoku Board", `Inline html tab should use document title, got ${srcdoc_view.tab_title}`);

  const workspace_view = buildBrowserSessionView({
    event: base_event,
    preview: null,
    query: "gomoku.html",
    target: "gomoku.html",
  });
  assert(workspace_view.iframe_url === "/nexus/v1/agents/agent-stage/workspace/site/gomoku.html", `Workspace html should build directory site URL, got ${workspace_view.iframe_url}`);
  assert(workspace_view.page_kind === "workspace", `Workspace html should be workspace page kind, got ${workspace_view.page_kind}`);
  assert(workspace_view.source_label === "工作区", `Workspace html source should be workspace, got ${workspace_view.source_label}`);
  assert(workspace_view.tab_title === "gomoku.html", `Workspace html tab should use basename, got ${workspace_view.tab_title}`);

  const remote_view = buildBrowserSessionView({
    event: { ...base_event, phase: "running", target: "https://example.com" },
    preview: null,
    query: "https://example.com",
  });
  assert(remote_view.iframe_url === "https://example.com", `Remote URL should open in the stage browser, got ${remote_view.iframe_url}`);
  assert(remote_view.page_kind === "web", `Remote URL should be web page kind, got ${remote_view.page_kind}`);
  assert(remote_view.source_label === "网页", `Remote URL source should be web, got ${remote_view.source_label}`);
  assert(remote_view.status.label === "正在访问", `Running URL should report a truthful access state, got ${remote_view.status.label}`);
  assert(remote_view.tab_title === "example.com", `Remote URL tab should use hostname, got ${remote_view.tab_title}`);

  const search_view = buildBrowserSessionView({
    event: { ...base_event, target: "nexus mac stage" },
    preview: null,
    query: "nexus mac stage",
  });
  assert(search_view.page_kind === "search", `Search fallback should be search page kind, got ${search_view.page_kind}`);
  assert(search_view.tab_title === "搜索：nexus mac stage", `Search fallback tab should expose search intent, got ${search_view.tab_title}`);
}

function verify_finder_details_reflect_selected_workspace_item(now) {
  const items = [{
    id: "workspace-html",
    agent_id: "agent-stage",
    path: "src/gomoku.html",
    status: "updated",
    version: 3,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-html",
    event_type: "file_write_end",
    live_content: "<main>\n  <h1>Gomoku</h1>\n</main>\n",
    updated_at: now,
  }, {
    id: "workspace-css",
    agent_id: "agent-stage",
    path: "src/style.css",
    status: "updated",
    version: 2,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-css",
    event_type: "file_write_end",
    live_content: "body { margin: 0; }\n",
    updated_at: now,
  }];

  const selected = resolveFinderSelectedItem(items, "src/gomoku.html");
  assert(selected?.path === "src/gomoku.html", `Finder should resolve selected file, got ${selected?.path}`);
  assert(finderFileKindLabel("src/gomoku.html") === "网页文件", "Finder should label html files as web files");
  assert(finderFileKindLabel("src/app.tsx") === "JavaScript 源代码", "Finder should label tsx files as JavaScript source");
  const previewLines = finderPreviewLines(selected);
  assert(previewLines.length === 3, `Finder preview should preserve non-empty live content lines, got ${previewLines.length}`);
  assert(previewLines[1].includes("Gomoku"), `Finder preview should include selected file content, got ${previewLines[1]}`);
}

function verify_finder_session_view(now) {
  const base_event = {
    id: "finder-event",
    session_key: "session:stage",
    round_id: "round-finder",
    agent_id: "agent-stage",
    message_id: "msg-finder",
    kind: "file_write",
    surface: "code",
    phase: "running",
    title: "写入文件",
    target: "src/gomoku.html",
    updated_at: now,
  };
  const items = [{
    id: "workspace-html",
    agent_id: "agent-stage",
    path: "src/gomoku.html",
    status: "updated",
    version: 3,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-html",
    event_type: "file_write_end",
    live_content: "<main>\n  <h1>Gomoku</h1>\n</main>\n",
    updated_at: now,
  }, {
    id: "workspace-css",
    agent_id: "agent-stage",
    path: "src/styles/main.css",
    status: "writing",
    version: 2,
    source: "agent",
    session_key: "session:stage",
    tool_use_id: "tool-css",
    event_type: "file_write_progress",
    live_content: "body { margin: 0; }\n",
    updated_at: now,
  }, {
    id: "workspace-readme",
    agent_id: "agent-stage",
    path: "README.md",
    status: "idle",
    version: 1,
    source: "agent",
    session_key: "session:stage",
    event_type: "file_write_end",
    updated_at: now,
  }];

  const view = buildFinderSessionView({
    active_path: "src/gomoku.html",
    event: base_event,
    items,
  });
  assert(view.item_count === 3, `Finder session should expose item count, got ${view.item_count}`);
  assert(view.changed_count === 2, `Finder session should count updated and writing items, got ${view.changed_count}`);
  assert(view.selected_item?.path === "src/gomoku.html", `Finder session should resolve selected item, got ${view.selected_item?.path}`);
  assert(view.previewLines[1].includes("Gomoku"), `Finder session preview should use selected live content, got ${view.previewLines[1]}`);
  assert(view.path_parts.join("/") === "src/gomoku.html", `Finder session should expose path parts, got ${view.path_parts.join("/")}`);
  assert(view.rows.some((row) => row.path === "src" && row.type === "folder"), "Finder session should include workspace folder rows");
  assert(view.rows.some((row) => row.path === "src/styles/main.css" && row.depth === 2), "Finder session should include nested file rows");
  assert(workspaceStatusLabel("writing") === "写入中", "Finder session should label writing files");

  const fallback = buildFinderSessionView({
    active_path: null,
    event: { ...base_event, target: "workspace/untitled.html" },
    items: [],
  });
  assert(fallback.display_items[0]?.path === "workspace/untitled.html", `Finder fallback should use event target, got ${fallback.display_items[0]?.path}`);
  assert(fallback.display_items[0]?.status === "writing", `Running Finder fallback should show writing status, got ${fallback.display_items[0]?.status}`);
}

function verify_console_events_use_mac_app_subsystems(now) {
  const base_event = {
    id: "console-event",
    session_key: "session:stage",
    round_id: "round-console",
    agent_id: "agent-stage",
    message_id: "msg-console",
    kind: "unknown",
    surface: "summary",
    phase: "done",
    updated_at: now,
  };

  assert(consoleEventLevel({ ...base_event, phase: "done" }.phase) === "INFO", "Console should map done events to INFO");
  assert(consoleEventLevel({ ...base_event, phase: "running" }.phase) === "INFO", "Console should map running events to INFO");
  assert(consoleEventLevel({ ...base_event, phase: "waiting" }.phase) === "NOTICE", "Console should map waiting events to NOTICE");
  assert(consoleEventLevel({ ...base_event, phase: "error" }.phase) === "ERROR", "Console should map error events to ERROR");
  assert(consoleEventSubsystem({ ...base_event, surface: "terminal" }) === "Terminal", "Console subsystem should use Terminal for command events");
  assert(consoleEventSubsystem({ ...base_event, surface: "web" }) === "Navi", "Console subsystem should use Navi for web events");
  assert(consoleEventSubsystem({ ...base_event, surface: "workspace" }) === "Finder", "Console subsystem should use Finder for workspace events");
  assert(consoleEventSubsystem({ ...base_event, surface: "editor" }) === "Code", "Console subsystem should use Code for editor events");

  const sources = collectManifestLogSources([
    { ...base_event, id: "terminal", surface: "terminal" },
    { ...base_event, id: "web", surface: "web" },
    { ...base_event, id: "editor", surface: "editor" },
  ]);
  assert(sources[0]?.label === "这台 Mac", `Console source list should begin with this Mac, got ${sources[0]?.label}`);
  assert(sources.some((source) => source.label === "Nexus" && source.count === 3), "Console source list should include Nexus desktop source");
  assert(sources.some((source) => source.label === "Terminal"), "Console source list should include Terminal source");
  assert(sources.some((source) => source.label === "Navi"), "Console source list should include Navi source");
  assert(sources.some((source) => source.label === "Code"), "Console source list should include Code source");
}

function verify_tasks_app_uses_real_task_fields(now) {
  const event = {
    agent_id: "agent-stage",
    id: "task-progress-real",
    input_preview: {
      description: "验证任务 App",
      last_tool_name: "Read",
      status: "running",
      task_id: "task-real-42",
      usage: { duration_ms: 2500, tool_uses: 2, total_tokens: 420 },
    },
    kind: "task_progress",
    phase: "running",
    round_id: "round-task-real",
    session_key: "session:stage",
    surface: "task",
    target: "task-real-42",
    title: "验证任务 App",
    tool_name: "TaskOutput",
    updated_at: now,
  };
  const session = buildTaskAppSession(event, [event]);
  const item = session.task_items[0];
  assert(item?.task_id === "task-real-42", `Tasks should preserve the real task id, got ${item?.task_id}`);
  assert(item?.last_tool_name === "Read", `Tasks should preserve the real last tool, got ${item?.last_tool_name}`);
  assert(item?.state === "running", `Tasks should preserve the real task state, got ${item?.state}`);
  assert(item?.usage.some((usage) => usage.label === "Tokens" && usage.value === "420"), "Tasks should show reported token usage");
  assert(!Object.hasOwn(item ?? {}, "pid"), "Tasks must not fabricate a process id");
  assert(!Object.hasOwn(item ?? {}, "cpu"), "Tasks must not fabricate CPU telemetry");
}

function verify_completed_manifest_keeps_terminal_window_identity(now) {
  const messages = [{
    role: "assistant",
    message_id: "msg-completed-terminal",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-completed-terminal",
    timestamp: now - 1500,
    is_complete: true,
    content: [
      {
        type: "tool_use",
        id: "tool-bash",
        name: "Bash",
        input: {
          command: "open gomoku.html",
        },
      },
      {
        type: "tool_result",
        tool_use_id: "tool-bash",
        content: "opened gomoku.html",
        is_error: false,
      },
    ],
    result_summary: {
      subtype: "success",
      duration_ms: 1500,
      duration_api_ms: 1200,
      num_turns: 1,
      result: "opened",
      is_error: false,
      timestamp: now - 100,
    },
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events: [],
  });
  const desktop = planOperationDesktop({
    event: snapshot.active_event,
    snapshot,
  });
  const terminal_window = desktop.windows.find((window) => window.kind === "terminal");
  assert(terminal_window, "completed stage should keep terminal window when the round had command events");
  assert(terminal_window.payload.event.surface === "terminal", `terminal window should keep terminal event identity, got ${terminal_window.payload.event.surface}`);
  assert(terminal_window.payload.event.tool_name === "Bash", `terminal window should keep Bash event identity, got ${terminal_window.payload.event.tool_name}`);
}

function verify_pending_permissions_are_scoped_and_precise(now) {
  const messages = [{
    role: "assistant",
    message_id: "msg-permission",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-permission",
    timestamp: now - 1000,
    is_complete: false,
    content: [
      {
        type: "tool_use",
        id: "tool-ls",
        name: "Bash",
        input: {
          command: "ls",
        },
      },
      {
        type: "tool_use",
        id: "tool-pwd",
        name: "Bash",
        input: {
          command: "pwd",
        },
      },
    ],
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [{
      request_id: "permission-current",
      tool_name: "Bash",
      tool_input: {
        command: "pwd",
      },
      session_key: "session:stage",
      agent_id: "agent-stage",
      message_id: "msg-permission",
      risk_label: "medium",
      summary: "需要确认 pwd",
    }, {
      request_id: "permission-stale-session",
      tool_name: "Bash",
      tool_input: {
        command: "rm -rf old",
      },
      session_key: "session:old",
      agent_id: "agent-stage",
      message_id: "msg-permission",
      risk_label: "high",
      summary: "旧会话权限",
    }, {
      request_id: "permission-stale-agent",
      tool_name: "Write",
      tool_input: {
        file_path: "stale-agent.md",
      },
      session_key: "session:stage",
      agent_id: "agent-old",
      risk_label: "high",
      summary: "旧智能体权限",
    }, {
      request_id: "permission-unscoped",
      tool_name: "Edit",
      tool_input: {
        file_path: "unscoped.md",
      },
      risk_label: "medium",
      summary: "缺少归属的权限",
    }],
    live_round_ids: ["round-permission"],
    workspace_events: [],
  });

  const ls_event = snapshot.events.find((event) => event.tool_use_id === "tool-ls");
  const pwd_event = snapshot.events.find((event) => event.tool_use_id === "tool-pwd");
  assert(ls_event?.phase === "running", `unmatched Bash tool should keep running, got ${ls_event?.phase}`);
  assert(pwd_event?.phase === "waiting", `exact Bash permission should attach to pwd, got ${pwd_event?.phase}`);
  assert(pwd_event?.summary === "需要确认 pwd", "matched permission summary should be attached to the precise tool");
  assert(!snapshot.events.some((event) => event.id === "permission:permission-stale-session"), "permission from another session should not enter stage events");
  assert(!snapshot.events.some((event) => event.id === "permission:permission-stale-agent"), "permission from another agent should not enter stage events");
  assert(!snapshot.events.some((event) => event.id === "permission:permission-unscoped"), "unscoped permission should not enter a session-specific stage");
}

function verify_live_round_without_tool_events_stays_hidden(now) {
  const messages = [{
    role: "user",
    message_id: "msg-user",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-live",
    timestamp: now - 1000,
    content: "写一个五子棋小游戏",
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: ["round-live"],
    workspace_events: [],
  });
  assert(snapshot.active_event === null, "live round without tool events should not create a placeholder active event");
  assert(snapshot.events.length === 0, `live round without tool events should stay out of the stage timeline, got ${snapshot.events.length}`);
}

function verify_synthetic_error_summary(now) {
  const messages = [{
    role: "user",
    message_id: "msg-user-error",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-error",
    timestamp: now - 186000,
    content: "写一个五子棋小游戏",
  }, {
    role: "assistant",
    message_id: "msg-synthetic-error",
    session_key: "session:stage",
    agent_id: "agent-stage",
    round_id: "round-error",
    timestamp: now - 1000,
    is_complete: true,
    model: "<synthetic>",
    content: [{
      type: "text",
      text: "Failed to authenticate. API Error: 401",
    }],
    result_summary: {
      subtype: "success",
      duration_ms: 1000,
      duration_api_ms: 0,
      num_turns: 1,
      is_error: false,
      timestamp: now,
    },
  }];

  const snapshot = projectOperationSnapshot({
    key: "session:stage",
    session_key: "session:stage",
    agent_id: "agent-stage",
    messages,
    pending_permissions: [],
    live_round_ids: [],
    workspace_events: [],
  });
  assert(snapshot.active_event?.phase === "error", `synthetic API error should project as error, got ${snapshot.active_event?.phase}`);
  assert(snapshot.active_event?.title === "本轮执行异常", `synthetic API error title should be abnormal, got ${snapshot.active_event?.title}`);
  assert(snapshot.active_event?.evidence?.some((item) => item.type === "error"), "synthetic API error should keep error evidence");
  assert(snapshot.active_event?.result_preview?.is_error === true, "synthetic API error summary preview should be marked as error");
  assert(snapshot.active_event?.result_preview?.subtype === "error", `synthetic API error summary preview should use error subtype, got ${snapshot.active_event?.result_preview?.subtype}`);
  assert(snapshot.active_event?.started_at === now - 186000, "summary event should start from the first message in the round");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}
