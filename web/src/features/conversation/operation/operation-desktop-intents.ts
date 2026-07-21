import type { NexusOperationEvent } from "./operation-types";
import type { OperationRuntimeEvent } from "./operation-runtime-types";
import {
  DEFAULT_TARGET_KEYS,
  resolveOperationToolProfile,
} from "./operation-tool-catalog";
import { terminalProgressResultFromRuntimeDelta } from "./operation-terminal-progress";
import {
  readStageDisplayCommand,
  readStageOpenCommand,
} from "./operation-stage-open-command";
import { resolveOperationToolVisualContract } from "./operation-tool-visual-contract";

export type StageDesktopIntent =
  | { app: "finder"; action: "inspect_files"; event_id: string; target?: string | null }
  | { app: "code"; action: "inspect_file" | "edit_file"; event_id: string; target?: string | null }
  | { app: "terminal"; action: "run_command"; command?: string | null; event_id: string; target?: string | null }
  | { app: "browser"; action: "browse" | "preview_artifact"; event_id: string; query?: string | null; target?: string | null; url?: string | null }
  | { app: "preview"; action: "preview_artifact"; event_id: string; target?: string | null }
  | { app: "handoff"; action: "summarize_delivery"; event_id: string; target?: string | null }
  | { app: "tasks"; action: "track_task"; event_id: string; target?: string | null }
  | { app: "system"; action: "request_confirmation"; event_id: string; target?: string | null };

export interface BrowserOpenTarget {
  target: string;
  url: string | null;
}

export function deriveStageDesktopIntents(event: NexusOperationEvent): StageDesktopIntent[] {
  const visual_contract = resolveOperationToolVisualContract(event);
  const intents: StageDesktopIntent[] = [];

  if (visual_contract.group === "workspace_navigation") {
    intents.push({
      app: "finder",
      action: "inspect_files",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "image_viewer") {
    intents.push({
      app: "preview",
      action: "preview_artifact",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "workspace_reader") {
    intents.push({
      app: "code",
      action: "inspect_file",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "workspace_writer") {
    intents.push({
      app: "code",
      action: "edit_file",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "command_runner") {
    const raw_command = read_stage_input_string(event.input_preview, ["command", "cmd", "description"]) ?? event.target;
    const command = raw_command ? readStageDisplayCommand(raw_command) : raw_command;
    intents.push({
      app: "terminal",
      action: "run_command",
      command,
      event_id: event.id,
      target: event.target,
    });
    const open_target = readBrowserOpenTargetFromTerminalCommand(event);
    if (open_target && (event.phase === "running" || event.phase === "done")) {
      if (open_target.url || looks_like_html_target(open_target.target)) {
        intents.push({
          app: "browser",
          action: looks_like_html_target(open_target.target) ? "preview_artifact" : "browse",
          event_id: event.id,
          query: open_target.target,
          target: open_target.target,
          url: open_target.url,
        });
      } else {
        intents.push({
          app: "preview",
          action: "preview_artifact",
          event_id: event.id,
          target: open_target.target,
        });
      }
    }
  } else if (visual_contract.group === "web_browser") {
    const query = readStageBrowserQuery(event);
    intents.push({
      app: "browser",
      action: "browse",
      event_id: event.id,
      query,
      target: event.target,
      url: query && looksLikeUrl(query) ? query : null,
    });
  } else if (visual_contract.group === "task_planner") {
    intents.push({
      app: "tasks",
      action: "track_task",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "human_gate") {
    intents.push({
      app: "system",
      action: "request_confirmation",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "handoff") {
    intents.push({
      app: "handoff",
      action: "summarize_delivery",
      event_id: event.id,
      target: event.target,
    });
  } else if (visual_contract.group === "knowledge_tool") {
    intents.push({
      app: "preview",
      action: "preview_artifact",
      event_id: event.id,
      target: event.target,
    });
  }

  if (
    (event.phase === "waiting" || event.permission_request_id) &&
    !intents.some((intent) => intent.app === "system")
  ) {
    intents.push({
      app: "system",
      action: "request_confirmation",
      event_id: event.id,
      target: event.target,
    });
  }

  return intents;
}

export function deriveStageDesktopIntentsFromRuntimeEvent(
  runtime_event: OperationRuntimeEvent,
): StageDesktopIntent[] {
  return deriveStageDesktopIntents(operationEventFromRuntimeEvent(runtime_event));
}

export function operationEventFromRuntimeEvent(
  runtime_event: OperationRuntimeEvent,
): NexusOperationEvent {
  const profile = resolveOperationToolProfile(runtime_event.tool_name);
  const raw_command = read_stage_input_string(runtime_event.input, ["command", "cmd"]);
  const stage_open = raw_command ? readStageOpenCommand(raw_command) : null;
  const target = runtime_event.artifact?.path
    ?? stage_open?.target
    ?? read_stage_input_string(runtime_event.input, profile.target_keys)
    ?? read_stage_input_string(runtime_event.input, DEFAULT_TARGET_KEYS)
    ?? runtime_event.tool_name
    ?? runtime_event.event_type;
  const is_permission = runtime_event.event_type === "permission_request" ||
    runtime_event.event_type === "permission_resolved";
  const is_handoff = runtime_event.event_type === "round_handoff";
  const is_artifact = runtime_event.event_type === "artifact_update";

  return {
    id: runtime_event.source_event_id ?? runtime_event.id,
    session_key: runtime_event.session_key ?? "",
    round_id: runtime_event.round_id,
    agent_id: runtime_event.agent_id,
    message_id: runtime_event.message_id,
    tool_use_id: runtime_event.tool_use_id,
    tool_name: runtime_event.tool_name,
    kind: is_handoff
      ? "round_summary"
      : is_permission
        ? "human_gate"
        : is_artifact
          ? "artifact_update"
          : profile.kind,
    surface: is_handoff
      ? "summary"
      : is_permission
        ? "conversation"
        : is_artifact
          ? "editor"
          : profile.surface,
    phase: runtime_event.phase,
    title: is_handoff
      ? "运行交付"
      : is_permission
        ? "等待用户确认"
        : profile.title,
    target,
    summary: summarize_runtime_label(runtime_event.delta) ?? summarize_runtime_label(runtime_event.result),
    input_preview: runtime_event.input,
    result_preview: runtime_event.result
      ?? terminalProgressResultFromRuntimeDelta(runtime_event.delta)
      ?? runtime_event.artifact?.preview
      ?? null,
    evidence: runtime_event.artifact?.path
      ? [{ type: "artifact", label: runtime_event.artifact.status ?? "artifact", value: runtime_event.artifact.path }]
      : undefined,
    permission_request_id: runtime_event.permission_request_id ?? null,
    permission_decision: runtime_event.permission_decision ?? null,
    permission_interaction_mode: runtime_event.permission_interaction_mode ?? null,
    duration_ms: runtime_event.duration_ms ?? null,
    started_at: runtime_event.duration_ms == null
      ? runtime_event.timestamp
      : runtime_event.timestamp - runtime_event.duration_ms,
    updated_at: runtime_event.timestamp,
  };
}

export function stageAppSessionIdForIntent(
  round_id: string,
  intent: StageDesktopIntent,
  normalize_id: (value: string) => string,
): string {
  if (intent.app === "finder") {
    return `${round_id}:finder`;
  }
  if (intent.app === "terminal") {
    return `${round_id}:terminal`;
  }
  if (intent.app === "browser") {
    return `${round_id}:browser`;
  }
  if (intent.app === "code") {
    return `${round_id}:document:${normalize_id(intent.target ?? "untitled")}`;
  }
  if (intent.app === "preview") {
    return `${round_id}:preview:${normalize_id(intent.target ?? "artifact")}`;
  }
  if (intent.app === "tasks") {
    return `${round_id}:tasks`;
  }
  if (intent.app === "system") {
    return `${round_id}:system-gate`;
  }
  if (intent.app === "handoff") {
    return `${round_id}:handoff`;
  }
  return `${round_id}:desktop`;
}

export function findStageDesktopIntent(
  event: NexusOperationEvent,
  app: StageDesktopIntent["app"],
): StageDesktopIntent | null {
  return deriveStageDesktopIntents(event).find((intent) => intent.app === app) ?? null;
}

export function readStageBrowserQuery(event: NexusOperationEvent): string | null {
  return read_stage_input_string(event.input_preview, ["url", "query", "q", "search_query", "prompt"])
    ?? event.target
    ?? null;
}

export function readBrowserOpenTargetFromTerminalCommand(event: NexusOperationEvent): BrowserOpenTarget | null {
  const command = read_stage_input_string(event.input_preview, ["command", "cmd", "description"]) ?? event.target;
  if (!command) {
    return null;
  }

  const stage_open = readStageOpenCommand(command);
  if (!stage_open) {
    return null;
  }
  return {
    target: stage_open.target,
    url: stage_open.url,
  };
}

function read_stage_input_string(
  input: Record<string, unknown> | null | undefined,
  keys: readonly string[],
): string | null {
  if (!input) {
    return null;
  }
  for (const key of keys) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
    if (typeof value === "number" || typeof value === "boolean") {
      return String(value);
    }
  }
  return null;
}

function summarize_runtime_label(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) {
    return value.trim().slice(0, 160);
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => summarize_runtime_label(item)).find(Boolean) ?? null;
  }
  if (value && typeof value === "object") {
    const record = value as Record<string, unknown>;
    for (const key of ["summary", "status", "content", "stdout", "stderr", "message", "error"]) {
      const label = summarize_runtime_label(record[key]);
      if (label) {
        return label;
      }
    }
  }
  return null;
}

function looks_like_html_target(value: string): boolean {
  return /\.(html?|xhtml)(?:[?#].*)?$/i.test(value);
}

function looksLikeUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}
