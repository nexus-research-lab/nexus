import type { NexusOperationEvent } from "./operation-types";
import type { OperationRuntimeEvent } from "./operation-runtime-types";
import {
  DEFAULT_TARGET_KEYS,
  resolve_operation_tool_profile,
} from "./operation-tool-catalog";
import { resolve_operation_tool_visual_contract } from "./operation-tool-visual-contract";

export type StageDesktopIntent =
  | { app: "finder"; action: "inspect_files"; event_id: string; target?: string | null }
  | { app: "code"; action: "inspect_file" | "edit_file"; event_id: string; target?: string | null }
  | { app: "terminal"; action: "run_command"; command?: string | null; event_id: string; target?: string | null }
  | { app: "browser"; action: "browse" | "preview_artifact"; event_id: string; query?: string | null; target?: string | null; url?: string | null }
  | { app: "preview"; action: "preview_artifact"; event_id: string; target?: string | null }
  | { app: "handoff"; action: "summarize_delivery"; event_id: string; target?: string | null }
  | { app: "activity"; action: "track_task"; event_id: string; target?: string | null }
  | { app: "system"; action: "request_confirmation"; event_id: string; target?: string | null }
  | { app: "nexus"; action: "run_tool"; event_id: string; target?: string | null };

export interface BrowserOpenTarget {
  target: string;
  url: string | null;
}

export function derive_stage_desktop_intents(event: NexusOperationEvent): StageDesktopIntent[] {
  const visual_contract = resolve_operation_tool_visual_contract(event);
  const intents: StageDesktopIntent[] = [];

  if (visual_contract.group === "workspace_navigation") {
    intents.push({
      app: "finder",
      action: "inspect_files",
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
    const command = read_stage_input_string(event.input_preview, ["command", "cmd", "description"]) ?? event.target;
    intents.push({
      app: "terminal",
      action: "run_command",
      command,
      event_id: event.id,
      target: event.target,
    });
    const open_target = read_browser_open_target_from_terminal_command(event);
    if (open_target) {
      intents.push({
        app: "browser",
        action: looks_like_html_target(open_target.target) ? "preview_artifact" : "browse",
        event_id: event.id,
        query: open_target.target,
        target: open_target.target,
        url: open_target.url,
      });
    }
  } else if (visual_contract.group === "web_browser") {
    const query = read_stage_browser_query(event);
    intents.push({
      app: "browser",
      action: "browse",
      event_id: event.id,
      query,
      target: event.target,
      url: query && looks_like_url(query) ? query : null,
    });
  } else if (visual_contract.group === "task_planner") {
    intents.push({
      app: "activity",
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
  } else if (visual_contract.group === "generic_tool") {
    intents.push({
      app: "nexus",
      action: "run_tool",
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

export function derive_stage_desktop_intents_from_runtime_event(
  runtime_event: OperationRuntimeEvent,
): StageDesktopIntent[] {
  return derive_stage_desktop_intents(operation_event_from_runtime_event(runtime_event));
}

export function operation_event_from_runtime_event(
  runtime_event: OperationRuntimeEvent,
): NexusOperationEvent {
  const profile = resolve_operation_tool_profile(runtime_event.tool_name);
  const target = runtime_event.artifact?.path
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
    result_preview: runtime_event.result ?? runtime_event.artifact?.preview ?? null,
    evidence: runtime_event.artifact?.path
      ? [{ type: "artifact", label: runtime_event.artifact.status ?? "artifact", value: runtime_event.artifact.path }]
      : undefined,
    permission_request_id: runtime_event.permission_request_id ?? null,
    permission_decision: runtime_event.permission_decision ?? null,
    permission_interaction_mode: runtime_event.permission_interaction_mode ?? null,
    updated_at: runtime_event.timestamp,
  };
}

export function stage_app_session_id_for_intent(
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
  if (intent.app === "activity") {
    return `${round_id}:task-board`;
  }
  if (intent.app === "system") {
    return `${round_id}:system-gate`;
  }
  if (intent.app === "handoff") {
    return `${round_id}:handoff`;
  }
  return `${round_id}:tool:${normalize_id(intent.target ?? intent.event_id)}`;
}

export function find_stage_desktop_intent(
  event: NexusOperationEvent,
  app: StageDesktopIntent["app"],
): StageDesktopIntent | null {
  return derive_stage_desktop_intents(event).find((intent) => intent.app === app) ?? null;
}

export function read_stage_browser_query(event: NexusOperationEvent): string | null {
  return read_stage_input_string(event.input_preview, ["url", "query", "q", "search_query", "prompt"])
    ?? event.target
    ?? null;
}

export function read_browser_open_target_from_terminal_command(event: NexusOperationEvent): BrowserOpenTarget | null {
  const command = read_stage_input_string(event.input_preview, ["command", "cmd", "description"]) ?? event.target;
  if (!command) {
    return null;
  }

  const target = extract_open_command_target(command);
  if (!target) {
    return null;
  }

  return {
    target,
    url: looks_like_url(target) ? target : null,
  };
}

function extract_open_command_target(command: string): string | null {
  const normalized = command.trim();
  if (!normalized) {
    return null;
  }

  const direct_url = normalized.match(/\bhttps?:\/\/[^\s'"`]+/i)?.[0]?.replace(/[),.;]+$/, "");
  const direct_html = normalized.match(/(?:^|\s)([^\s'"`]+\.x?html?)(?:\s|$)/i)?.[1]?.replace(/[),.;]+$/, "");
  const open_match = normalized.match(/\b(?:open|xdg-open|start)\b\s+(?:-[^\s]+\s+(?:"[^"]+"\s+|'[^']+'\s+)?)*["']?([^"'\s]+)["']?/i);
  const candidate = open_match?.[1] ?? direct_url ?? direct_html ?? null;
  if (!candidate || candidate.startsWith("-")) {
    return null;
  }
  if (!looks_like_url(candidate) && !looks_like_html_target(candidate)) {
    return null;
  }
  return candidate.replace(/^file:\/\//i, "");
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

function looks_like_url(value: string): boolean {
  return /^https?:\/\//i.test(value);
}
