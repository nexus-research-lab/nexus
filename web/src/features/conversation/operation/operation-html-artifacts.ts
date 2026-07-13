import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";
import { readBrowserOpenTargetFromTerminalCommand } from "./operation-desktop-intents";

export interface OperationHtmlArtifact {
  path: string;
  live_content: string | null;
}

export function findOperationHtmlArtifact(
  snapshot: NexusOperationSnapshot | null,
  events: NexusOperationEvent[],
): OperationHtmlArtifact | null {
  const html_targets = new Set<string>();
  const round_tool_use_ids = new Set<string>();
  let candidate_path: string | null = null;
  for (const event of [...events].reverse()) {
    if (event.tool_use_id) {
      round_tool_use_ids.add(event.tool_use_id);
    }
    const html_target = read_event_html_target(event);
    if (!html_target) {
      continue;
    }
    html_targets.add(html_target);
    candidate_path ??= html_target;
    const content = read_event_html_content(event);
    if (content) {
      return {
        path: html_target,
        live_content: content,
      };
    }
  }

  const workspace_items = [...(snapshot?.workspace_events ?? [])].reverse();
  const workspace_artifact = workspace_items.find((item) => (
    html_targets.has(item.path) &&
    looks_like_html_path(item.path)
  )) ?? workspace_items.find((item) => (
    Boolean(item.tool_use_id && round_tool_use_ids.has(item.tool_use_id)) &&
    looks_like_html_path(item.path)
  )) ?? (!candidate_path ? workspace_items.find((item) => looks_like_html_path(item.path)) : null);
  if (workspace_artifact) {
    return {
      path: workspace_artifact.path,
      live_content: workspace_artifact.live_content ?? null,
    };
  }

  if (candidate_path) {
    return {
      path: candidate_path,
      live_content: null,
    };
  }

  return null;
}

function read_event_html_target(event: NexusOperationEvent): string | null {
  const open_target = readBrowserOpenTargetFromTerminalCommand(event);
  if (open_target?.target && looks_like_html_path(open_target.target)) {
    return open_target.target;
  }
  if (event.target && looks_like_html_path(event.target)) {
    return event.target;
  }
  return null;
}

function read_event_html_content(event: NexusOperationEvent): string | null {
  return readInputString(event.input_preview, ["content", "text", "body"])
    ?? (typeof event.result_preview === "string" && looks_like_html_content(event.result_preview)
      ? event.result_preview
      : null);
}

function readInputString(
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
  }
  return null;
}

function looks_like_html_path(path: string): boolean {
  return /\.(html?|xhtml)$/i.test(path);
}

function looks_like_html_content(value: string): boolean {
  return /<html|<!doctype|<script/i.test(value);
}
