import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";
import {
  deriveStageDesktopIntents,
  findStageDesktopIntent,
  readBrowserOpenTargetFromTerminalCommand,
  readStageBrowserQuery,
  stageAppSessionIdForIntent,
} from "./operation-desktop-intents";
import type {
  OperationDesktopState,
  StageHandoffSummary,
  StageWindowState,
} from "./operation-desktop-types";
import {
  collectOperationFileContext,
  fallbackWindowKindForFileEvent,
  operationWorkspaceTargetsMatch,
  resolveOperationWorkspaceTarget,
  windowKindForFileTarget,
} from "./operation-file-documents";
import { findOperationHtmlArtifact } from "./operation-html-artifacts";
import { buildOperationContinuationBrief } from "./operation-stage-experience";
import {
  preferredWindowKindsForEvent,
  resolveOperationFocusTarget,
} from "./operation-scene-focus";
import {
  basename,
  collectRoundEvents,
  looksLikeUrl,
  normalizeWindowId,
  previewLines,
} from "./operation-scene-planner-helpers";
import {
  buildOperationTerminalLines,
  readTerminalCommand,
} from "./operation-terminal-lines";
import { collectTerminalSessionEvents } from "./operation-terminal-session-events";
import {
  isDesktopToolActivityEvent,
  isRoundReviewEvent,
  shouldOpenFinderWindow,
  shouldOpenHtmlBrowserWindow,
  supportingWindowPhase,
} from "./operation-scene-window-policy";
import {
  buildOperationStageWindow,
} from "./operation-scene-window-builder";
export function planOperationDesktop({
  event,
  snapshot,
}: {
  event: NexusOperationEvent;
  snapshot: NexusOperationSnapshot | null;
}): OperationDesktopState {
  const windows = build_windows(event, snapshot);
  const active_window = resolve_active_window(windows);

  return {
    active_window_id: active_window?.id ?? null,
    surface: event.surface,
    phase: event.phase,
    windows,
    minimized: windows.filter((window) => window.phase === "minimized"),
    artifacts: windows.filter((window) => window.layout === "artifact"),
  };
}

function resolve_active_window(windows: StageWindowState[]): StageWindowState | null {
  const focused_windows = windows.filter((window) => window.phase === "focused");
  if (!focused_windows.length) {
    return windows[0] ?? null;
  }
  return [...focused_windows].sort((left, right) => right.z - left.z)[0] ?? null;
}

export function resolveOperationEventWindowId(
  event: NexusOperationEvent,
  windows: StageWindowState[],
): string | null {
  const related_windows = windows.filter((window) => (
    window.payload.event.id === event.id ||
    window.payload.related_events?.some((item) => item.id === event.id) ||
    Boolean(event.tool_use_id && (
      window.payload.event.tool_use_id === event.tool_use_id ||
      window.payload.related_events?.some((item) => item.tool_use_id === event.tool_use_id)
    ))
  ));
  const preferred_kind = preferredWindowKindsForEvent(event);
  const preferred_window = related_windows.find((window) => preferred_kind.includes(window.kind));
  if (preferred_window) {
    return preferred_window.id;
  }

  const exact_event_window = related_windows.find((window) => window.payload.event.id === event.id);
  if (exact_event_window) {
    return exact_event_window.id;
  }

  const target_window = event.target
    ? related_windows.find((window) => (
      window.target === event.target ||
      window.payload.target === event.target ||
      window.title === event.target
    ))
    : null;
  if (target_window) {
    return target_window.id;
  }

  const related_non_inspector = related_windows.find((window) => (
    window.kind !== "evidence" && window.kind !== "summary"
  ));
  if (related_non_inspector) {
    return related_non_inspector.id;
  }

  if (event.kind === "round_summary" || event.surface === "summary") {
    return windows.find((window) => (
      window.phase === "focused" &&
      window.kind !== "handoff" &&
      window.kind !== "run_manifest"
    ))?.id ?? windows.find((window) => (
      window.kind !== "handoff" &&
      window.kind !== "run_manifest"
    ))?.id ?? null;
  }

  return related_windows[0]?.id ?? null;
}

function build_windows(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
): StageWindowState[] {
  const round_events = collectRoundEvents(event, snapshot);
  const terminal_events = collectTerminalSessionEvents(event, snapshot, round_events);
  const web_events = round_events.filter((item) => item.surface === "web");
  const task_events = round_events.filter((item) => item.surface === "task");
  const tool_activity_events = round_events.filter(isDesktopToolActivityEvent);
  const file_context = collectOperationFileContext(event, snapshot, round_events);
  const html_artifact = findOperationHtmlArtifact(snapshot, round_events);
  const active_intents = deriveStageDesktopIntents(event);
  const open_browser_target = active_intents.some((intent) => intent.app === "browser")
    ? readBrowserOpenTargetFromTerminalCommand(event)
    : null;
  const resolved_browser_target = open_browser_target
    ? {
        ...open_browser_target,
        target: open_browser_target.url
          ?? resolveOperationWorkspaceTarget(open_browser_target.target, snapshot?.workspace_events ?? []),
      }
    : null;
  const open_preview_target = active_intents.some((intent) => intent.app === "preview")
    ? readBrowserOpenTargetFromTerminalCommand(event)
    : null;
  const resolved_preview_target = open_preview_target
    ? resolveOperationWorkspaceTarget(open_preview_target.target, snapshot?.workspace_events ?? [])
    : null;
  const is_review_event = isRoundReviewEvent(event);
  const focus_target = resolveOperationFocusTarget(event, {
    has_file: Boolean(file_context.latest_file_target || resolved_preview_target),
    has_html_artifact: Boolean(html_artifact || resolved_browser_target),
    has_task: task_events.length > 0,
    has_terminal: terminal_events.length > 0,
    has_web: web_events.length > 0,
    opens_browser: active_intents.some((intent) => intent.app === "browser"),
    opens_preview: active_intents.some((intent) => intent.app === "preview"),
  });
  const windows: StageWindowState[] = [];

  if (event.surface === "conversation" && tool_activity_events.length === 0) {
    return [];
  }

  if (
    shouldOpenFinderWindow(event, {
      file_document_count: file_context.file_documents.length,
      workspace_item_count: file_context.workspace_items.length,
    }) && (
      file_context.workspace_items.length > 0 ||
      file_context.file_documents.length > 0 ||
      file_context.latest_file_target
    )
  ) {
    const finder_intent = findStageDesktopIntent(file_context.latest_file_event ?? event, "finder");
    windows.push(buildOperationStageWindow(file_context.latest_file_event ?? event, snapshot, {
      id: "finder",
      session_id: finder_intent
        ? stageAppSessionIdForIntent(event.round_id, finder_intent, normalizeWindowId)
        : `${event.round_id}:finder`,
      kind: "finder",
      title: "工作区",
      layout: "secondary",
      phase: supportingWindowPhase("finder", focus_target === "finder", {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: focus_target === "finder" ? 34 : 14,
      payload: {
        workspace_items: file_context.workspace_items,
        related_events: round_events,
        target: file_context.latest_file_target ?? event.target,
        preview: event.input_preview ?? event.result_preview,
      },
    }));
  }

  file_context.file_documents.forEach((document, index) => {
    const is_focused_document = focus_target === "document" && (
      document.target === file_context.latest_file_target ||
      document.event.id === event.id
    );
    const document_kind = windowKindForFileTarget(
      document.target,
      fallbackWindowKindForFileEvent(document.event),
    );
    const document_intent = findStageDesktopIntent(document.event, "code") ?? {
      app: "code" as const,
      action: "inspect_file" as const,
      event_id: document.event.id,
      target: document.target,
    };
    windows.push(buildOperationStageWindow(document.event, snapshot, {
      id: `document:${normalizeWindowId(document.target)}`,
      session_id: stageAppSessionIdForIntent(event.round_id, document_intent, normalizeWindowId),
      kind: document_kind,
      title: document.target,
      layout: "primary",
      phase: supportingWindowPhase(document_kind, is_focused_document, {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: is_focused_document ? 36 : 20 - index,
      payload: {
        diff_stats: document.workspace_item?.diff_stats ?? null,
        preview: document.preview,
        related_events: document.related_events,
        summary: document.event.summary ?? event.summary,
        target: document.target,
        workspace_preview: true,
      },
    }));
  });

  const has_matching_document = resolved_preview_target
    ? file_context.file_documents.some((document) => (
      operationWorkspaceTargetsMatch(document.target, resolved_preview_target)
    ))
    : false;
  if (resolved_preview_target && !has_matching_document) {
    const preview_intent = active_intents.find((intent) => intent.app === "preview");
    const preview_kind = windowKindForFileTarget(resolved_preview_target, "code_editor");
    windows.push(buildOperationStageWindow(event, snapshot, {
      id: `document:${normalizeWindowId(resolved_preview_target)}`,
      session_id: preview_intent
        ? stageAppSessionIdForIntent(event.round_id, preview_intent, normalizeWindowId)
        : `${event.round_id}:preview:${normalizeWindowId(resolved_preview_target)}`,
      kind: preview_kind,
      title: resolved_preview_target,
      layout: "primary",
      phase: supportingWindowPhase(preview_kind, focus_target === "document", {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: focus_target === "document" ? 38 : 21,
      payload: {
        preview: snapshot?.workspace_events.find((item) => (
          operationWorkspaceTargetsMatch(item.path, resolved_preview_target)
        ))?.live_content ?? null,
        related_events: round_events,
        summary: `打开 ${resolved_preview_target}`,
        target: resolved_preview_target,
        workspace_preview: true,
      },
    }));
  }

  if (terminal_events.length > 0) {
    const terminal_event = terminal_events.at(-1) ?? event;
    const terminal_command_event = [...terminal_events].reverse().find((item) => (
      item.tool_name === "Bash" || item.kind === "command_run"
    )) ?? terminal_event;
    const terminal_command = readTerminalCommand(terminal_command_event);
    const terminal_intent = findStageDesktopIntent(terminal_event, "terminal");
    const terminal_lines = buildOperationTerminalLines(terminal_events);
    windows.push(buildOperationStageWindow(terminal_event, snapshot, {
      id: "terminal",
      session_id: terminal_intent
        ? stageAppSessionIdForIntent(event.round_id, terminal_intent, normalizeWindowId)
        : `${event.round_id}:terminal`,
      kind: "terminal",
      title: terminal_command || terminal_event.target || "终端",
      layout: "terminal",
      phase: supportingWindowPhase("terminal", focus_target === "terminal", {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: focus_target === "terminal" ? 36 : 18,
      payload: {
        command: terminal_command,
        lines: terminal_lines,
        related_events: terminal_events,
      },
    }));
  }

  if (web_events.length > 0 || shouldOpenHtmlBrowserWindow(event, Boolean(html_artifact)) || resolved_browser_target) {
    const web_event = web_events.at(-1) ?? event;
    const browser_target = resolved_browser_target?.target ?? html_artifact?.path ?? web_event.target;
    const browser_artifact = html_artifact && browser_target
      && operationWorkspaceTargetsMatch(html_artifact.path, browser_target)
      ? html_artifact
      : null;
    const browser_intent = findStageDesktopIntent(web_event, "browser") ?? (
      browser_target
        ? {
          app: "browser" as const,
          action: browser_artifact ? "preview_artifact" as const : "browse" as const,
          event_id: web_event.id,
          query: browser_target,
          target: browser_target,
          url: resolved_browser_target?.url ?? null,
        }
        : null
    );
    const query = resolved_browser_target?.target
      ?? (browser_artifact
        ? basename(browser_artifact.path)
        : null)
        ?? readStageBrowserQuery(web_event)
        ?? "web";
    const lines = previewLines(web_event.result_preview ?? web_event.summary, 8);
    windows.push(buildOperationStageWindow(web_event, snapshot, {
      id: browser_artifact ? `browser:${normalizeWindowId(browser_artifact.path)}` : "browser",
      session_id: browser_intent
        ? stageAppSessionIdForIntent(event.round_id, browser_intent, normalizeWindowId)
        : `${event.round_id}:browser`,
      kind: "browser",
      title: browser_artifact ? basename(browser_artifact.path) : query,
      layout: "primary",
      phase: supportingWindowPhase("browser", focus_target === "browser", {
        has_browser_artifact: Boolean(html_artifact || resolved_browser_target),
        is_review_event,
      }),
      z: focus_target === "browser" ? 38 : 22,
      payload: {
        lines,
        preview: web_event.result_preview ?? web_event.summary,
        query,
        related_events: web_events,
        srcdoc: browser_artifact?.live_content ?? null,
        target: browser_target,
        url: resolved_browser_target?.url ?? (looksLikeUrl(query) ? query : null),
      },
    }));
  }

  if (event.surface === "knowledge") {
    windows.push(buildOperationStageWindow(event, snapshot, {
      id: `knowledge:${normalizeWindowId(event.target ?? event.tool_name ?? event.title)}`,
      kind: "markdown_reader",
      title: event.target ?? event.tool_name ?? event.title,
      layout: "primary",
      phase: supportingWindowPhase("markdown_reader", focus_target === "document", {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: focus_target === "document" ? 36 : 20,
      payload: {
        preview: event.result_preview ?? event.input_preview ?? event.summary,
        related_events: round_events,
        summary: event.summary,
        target: event.target ?? event.tool_name,
      },
    }));
  }

  if (task_events.length > 0) {
    const task_event = task_events.at(-1) ?? event;
    const task_intent = findStageDesktopIntent(task_event, "activity");
    windows.push(buildOperationStageWindow(task_event, snapshot, {
      id: "task-board",
      session_id: task_intent
        ? stageAppSessionIdForIntent(event.round_id, task_intent, normalizeWindowId)
        : `${event.round_id}:task-board`,
      kind: "task_board",
      title: task_event.target ?? task_event.tool_name ?? "Task",
      layout: "primary",
      phase: supportingWindowPhase("task_board", focus_target === "task", {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: focus_target === "task" ? 36 : 17,
      payload: {
        lines: previewLines(task_event.result_preview ?? task_event.input_preview ?? task_event.summary, 8),
        related_events: task_events,
      },
    }));
  }

  if (event.surface === "summary" || event.surface === "conversation" || (event.surface === "fallback" && windows.length === 0)) {
    if (windows.length === 0 && (
      event.kind === "round_summary" ||
      event.phase === "done" ||
      event.phase === "error" ||
      event.phase === "cancelled"
    )) {
      const is_successful_handoff = event.kind === "round_summary" && event.phase === "done";
      windows.push(buildOperationStageWindow(event, snapshot, {
        id: is_successful_handoff ? "handoff" : "run-manifest",
        session_id: is_successful_handoff ? `${event.round_id}:handoff` : `${event.round_id}:run-manifest`,
        kind: is_successful_handoff ? "handoff" : "run_manifest",
        title: event.phase === "error" ? "Nexus Console · 诊断" : is_successful_handoff ? "Nexus 交付台" : "Nexus Console",
        layout: "primary",
        phase: focus_target === "manifest" ? "focused" : "background",
        z: focus_target === "manifest" ? 42 : 24,
        payload: {
          evidence: [
            ...(event.evidence ?? []),
            ...(snapshot?.recent_evidence ?? []),
          ].slice(0, 8),
          preview: event.result_preview ?? event.summary ?? event.target,
          related_events: round_events,
          summary: event.summary,
          target: "run-manifest.md",
          handoff_summary: build_handoff_summary(event, round_events, snapshot),
        },
      }));
    }
  }

  return windows;
}

function build_handoff_summary(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
): StageHandoffSummary {
  return buildOperationContinuationBrief(event, events, snapshot);
}
