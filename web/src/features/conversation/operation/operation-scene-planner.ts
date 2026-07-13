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
  StageWindowKind,
  StageWindowLayout,
  StageWindowPayload,
  StageWindowPhase,
  StageWindowState,
} from "./operation-desktop-types";
import {
  collectOperationFileContext,
  fallbackWindowKindForFileEvent,
  windowKindForFileTarget,
} from "./operation-file-documents";
import { findOperationHtmlArtifact } from "./operation-html-artifacts";
import { buildOperationContinuationBrief } from "./operation-stage-experience";
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
  fallbackStageEventObjectLabel,
  fallbackStageEventTargetLabel,
  isLowSignalStageLabel,
} from "./operation-stage-labels";
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
  const preferred_kind = preferred_window_kind_for_event(event);
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
  const is_review_event = isRoundReviewEvent(event);
  const focus_target = resolve_focus_target(event, {
    has_file: Boolean(file_context.latest_file_target),
    has_html_artifact: Boolean(html_artifact || open_browser_target),
    has_task: task_events.length > 0,
    has_terminal: terminal_events.length > 0,
    has_web: web_events.length > 0,
    opens_browser: active_intents.some((intent) => intent.app === "browser"),
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
    windows.push(window_state(file_context.latest_file_event ?? event, snapshot, {
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
    windows.push(window_state(document.event, snapshot, {
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
      },
    }));
  });

  if (terminal_events.length > 0) {
    const terminal_event = terminal_events.at(-1) ?? event;
    const terminal_command_event = [...terminal_events].reverse().find((item) => (
      item.tool_name === "Bash" || item.kind === "command_run"
    )) ?? terminal_event;
    const terminal_command = readTerminalCommand(terminal_command_event);
    const terminal_intent = findStageDesktopIntent(terminal_event, "terminal");
    const terminal_lines = buildOperationTerminalLines(terminal_events);
    windows.push(window_state(terminal_event, snapshot, {
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

  if (web_events.length > 0 || shouldOpenHtmlBrowserWindow(event, Boolean(html_artifact)) || open_browser_target) {
    const web_event = web_events.at(-1) ?? event;
    const browser_target = html_artifact?.path ?? open_browser_target?.target ?? web_event.target;
    const browser_intent = findStageDesktopIntent(web_event, "browser") ?? (
      browser_target
        ? {
          app: "browser" as const,
          action: html_artifact ? "preview_artifact" as const : "browse" as const,
          event_id: web_event.id,
          query: browser_target,
          target: browser_target,
          url: open_browser_target?.url ?? null,
        }
        : null
    );
    const query = html_artifact
      ? basename(html_artifact.path)
      : open_browser_target?.target
        ?? readStageBrowserQuery(web_event)
        ?? "web";
    const lines = previewLines(web_event.result_preview ?? web_event.summary, 8);
    windows.push(window_state(web_event, snapshot, {
      id: html_artifact ? `browser:${normalizeWindowId(html_artifact.path)}` : "browser",
      session_id: browser_intent
        ? stageAppSessionIdForIntent(event.round_id, browser_intent, normalizeWindowId)
        : `${event.round_id}:browser`,
      kind: "browser",
      title: html_artifact ? basename(html_artifact.path) : query,
      layout: "primary",
      phase: supportingWindowPhase("browser", focus_target === "browser", {
        has_browser_artifact: Boolean(html_artifact),
        is_review_event,
      }),
      z: focus_target === "browser" ? 38 : 22,
      payload: {
        lines,
        preview: web_event.result_preview ?? web_event.summary,
        query,
        related_events: web_events,
        srcdoc: html_artifact?.live_content ?? null,
        target: browser_target,
        url: open_browser_target?.url ?? (looksLikeUrl(query) ? query : null),
      },
    }));
  }

  if (event.surface === "knowledge") {
    windows.push(window_state(event, snapshot, {
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
    windows.push(window_state(task_event, snapshot, {
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
      windows.push(window_state(event, snapshot, {
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

function window_state(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
  config: {
    id: string;
    session_id?: string;
    kind: StageWindowKind;
    title: string;
    layout: StageWindowLayout;
    phase: StageWindowPhase;
    z: number;
    payload?: Partial<StageWindowPayload>;
  },
): StageWindowState {
  const title = normalize_stage_window_title(event, config.title);
  const subtitle = normalize_stage_window_subtitle(event);
  const target = normalize_stage_window_target(event, config.payload?.target);
  return {
    id: config.session_id ?? `${event.id}:${config.id}`,
    kind: config.kind,
    title,
    subtitle,
    target,
    phase: config.phase,
    z: config.z,
    layout: config.layout,
    payload: {
      event,
      snapshot,
      summary: event.summary,
      target: event.target,
      ...config.payload,
    },
  };
}

function normalize_stage_window_title(event: NexusOperationEvent, title: string): string {
  if (!isLowSignalStageLabel(title)) {
    return title;
  }
  return fallbackStageEventObjectLabel(event);
}

function normalize_stage_window_subtitle(event: NexusOperationEvent): string | null {
  if (!event.summary || isLowSignalStageLabel(event.summary)) {
    return null;
  }
  return event.summary;
}

function normalize_stage_window_target(
  event: NexusOperationEvent,
  target: string | null | undefined,
): string | null {
  const candidate = target ?? event.target;
  if (!candidate) {
    return null;
  }
  if (!isLowSignalStageLabel(candidate)) {
    return candidate;
  }
  return fallbackStageEventTargetLabel(event);
}

function build_handoff_summary(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
): StageHandoffSummary {
  return buildOperationContinuationBrief(event, events, snapshot);
}

function resolve_focus_target(
  event: NexusOperationEvent,
  context: {
    has_file: boolean;
    has_html_artifact: boolean;
    has_task: boolean;
    has_terminal: boolean;
    has_web: boolean;
    opens_browser: boolean;
  },
): "browser" | "document" | "finder" | "manifest" | "summary" | "task" | "terminal" {
  if (event.phase === "waiting" && event.surface === "terminal" && context.has_terminal) {
    return "terminal";
  }
  if (event.phase === "waiting" || event.surface === "conversation") {
    return "summary";
  }
  if (event.kind === "round_summary" || (
    event.surface === "summary" &&
    (event.phase === "done" || event.phase === "error" || event.phase === "cancelled")
  )) {
    if (context.has_html_artifact || context.has_web) {
      return "browser";
    }
    if (context.has_file) {
      return "document";
    }
    if (context.has_terminal) {
      return "terminal";
    }
    if (context.has_task) {
      return "task";
    }
    return "manifest";
  }
  if (event.surface === "task" && context.has_task) {
    return "task";
  }
  if (context.opens_browser && (event.surface === "terminal" || event.surface === "web")) {
    return "browser";
  }
  if (event.surface === "knowledge") {
    return "document";
  }
  if (event.surface === "terminal" && event.phase === "done" && context.has_html_artifact) {
    return "browser";
  }
  if (event.surface === "terminal" && context.has_terminal) {
    return "terminal";
  }
  if (event.surface === "web" || (context.has_html_artifact && event.surface === "summary")) {
    return "browser";
  }
  if ((event.surface === "workspace" || event.surface === "editor") && context.has_file) {
    if (
      event.kind === "workspace_read" ||
      event.kind === "workspace_edit" ||
      event.kind === "artifact_update" ||
      event.surface === "editor"
    ) {
      return "document";
    }
    return "finder";
  }
  if (context.has_html_artifact) {
    return "browser";
  }
  if (context.has_file) {
    return "document";
  }
  return "summary";
}

function preferred_window_kind_for_event(event: NexusOperationEvent): StageWindowKind[] {
  if (event.surface === "terminal") {
    return ["terminal"];
  }
  if (event.surface === "web") {
    return ["browser"];
  }
  if (event.surface === "task") {
    return ["task_board"];
  }
  if (event.surface === "conversation") {
    return ["terminal", "finder", "browser"];
  }
  if (event.surface === "summary" || event.kind === "round_summary") {
    return ["handoff", "run_manifest", "summary"];
  }
  if (event.surface === "workspace") {
    if (
      event.kind === "workspace_read" ||
      event.kind === "workspace_edit" ||
      event.kind === "artifact_update"
    ) {
      return ["code_editor", "markdown_reader", "word_reader", "pdf_reader", "spreadsheet", "image_viewer", "finder"];
    }
    return ["finder", "code_editor", "markdown_reader", "word_reader", "pdf_reader", "spreadsheet", "image_viewer"];
  }
  if (event.surface === "editor" || event.surface === "knowledge") {
    return ["code_editor", "markdown_reader", "word_reader", "pdf_reader", "spreadsheet", "image_viewer"];
  }
  return ["summary"];
}
