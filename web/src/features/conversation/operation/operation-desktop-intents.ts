import type { NexusOperationEvent } from "./operation-types";
import { resolve_operation_tool_profile } from "./operation-tool-catalog";

export type StageDesktopIntent =
  | { app: "finder"; action: "inspect_files"; event_id: string; target?: string | null }
  | { app: "code"; action: "inspect_file" | "edit_file"; event_id: string; target?: string | null }
  | { app: "terminal"; action: "run_command"; command?: string | null; event_id: string; target?: string | null }
  | { app: "browser"; action: "browse" | "preview_artifact"; event_id: string; query?: string | null; target?: string | null; url?: string | null }
  | { app: "preview"; action: "preview_artifact"; event_id: string; target?: string | null }
  | { app: "handoff"; action: "summarize_delivery"; event_id: string; target?: string | null }
  | { app: "activity"; action: "track_task"; event_id: string; target?: string | null }
  | { app: "nexus"; action: "run_tool"; event_id: string; target?: string | null };

export interface BrowserOpenTarget {
  target: string;
  url: string | null;
}

export function derive_stage_desktop_intents(event: NexusOperationEvent): StageDesktopIntent[] {
  const profile = resolve_operation_tool_profile(event.tool_name, event.kind, event.surface);
  const intents: StageDesktopIntent[] = [];

  if (profile.action === "list" || profile.action === "search") {
    intents.push({
      app: "finder",
      action: "inspect_files",
      event_id: event.id,
      target: event.target,
    });
  } else if (profile.action === "read") {
    intents.push({
      app: "code",
      action: "inspect_file",
      event_id: event.id,
      target: event.target,
    });
  } else if (profile.action === "create" || profile.action === "edit") {
    intents.push({
      app: "code",
      action: "edit_file",
      event_id: event.id,
      target: event.target,
    });
  } else if (profile.action === "run" || profile.action === "stop") {
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
  } else if (profile.action === "web_search" || profile.action === "web_fetch") {
    const query = read_stage_browser_query(event);
    intents.push({
      app: "browser",
      action: "browse",
      event_id: event.id,
      query,
      target: event.target,
      url: query && looks_like_url(query) ? query : null,
    });
  } else if (profile.action === "task" || profile.action === "task_progress") {
    intents.push({
      app: "activity",
      action: "track_task",
      event_id: event.id,
      target: event.target,
    });
  } else if (profile.action === "summary" || event.kind === "round_summary") {
    intents.push({
      app: "handoff",
      action: "summarize_delivery",
      event_id: event.id,
      target: event.target,
    });
  } else if (event.surface === "knowledge") {
    intents.push({
      app: "preview",
      action: "preview_artifact",
      event_id: event.id,
      target: event.target,
    });
  } else if (event.surface === "fallback") {
    intents.push({
      app: "nexus",
      action: "run_tool",
      event_id: event.id,
      target: event.target,
    });
  }

  return intents;
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
  keys: string[],
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

function looks_like_html_target(value: string): boolean {
  return /\.(html?|xhtml)(?:[?#].*)?$/i.test(value);
}

function looks_like_url(value: string): boolean {
  return /^https?:\/\//i.test(value);
}
