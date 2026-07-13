import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";

const SESSION_ID_KEYS = ["shell_id", "shellId", "task_id", "taskId"] as const;
const RESULT_SESSION_ID_KEYS = [
  ...SESSION_ID_KEYS,
  "background_task_id",
  "backgroundTaskId",
] as const;

export function collectTerminalSessionEvents(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
  roundEvents: NexusOperationEvent[],
): NexusOperationEvent[] {
  const currentEvents = roundEvents.filter((item) => (
    isTerminalCommandEvent(item) || isTerminalControlEvent(item)
  ));
  if (!isTerminalControlEvent(event)) {
    return currentEvents;
  }

  const targetSessionId = readTerminalControlSessionId(event);
  if (!targetSessionId || currentEvents.some((item) => (
    isTerminalCommandEvent(item)
    && readTerminalCommandSessionId(item) === targetSessionId
  ))) {
    return currentEvents;
  }

  const eventTime = event.started_at ?? event.updated_at;
  const previousCommand = snapshot?.events
    .filter((item) => (
      isTerminalCommandEvent(item)
      && (item.started_at ?? item.updated_at) <= eventTime
      && readTerminalCommandSessionId(item) === targetSessionId
    ))
    .sort(compareEvents)
    .at(-1);
  if (!previousCommand) {
    return currentEvents;
  }

  return dedupeEvents([previousCommand, ...currentEvents]).sort(compareEvents);
}

export function isTerminalCommandEvent(event: NexusOperationEvent): boolean {
  return event.kind === "command_run" && normalizeTerminalToolName(event.tool_name) === "bash";
}

export function isTerminalControlEvent(event: NexusOperationEvent): boolean {
  return event.kind === "command_stop" && normalizeTerminalToolName(event.tool_name) === "killshell";
}

export function readTerminalCommandSessionId(event: NexusOperationEvent): string | null {
  return readInputString(event.input_preview, SESSION_ID_KEYS)
    ?? readResultString(event.result_preview, RESULT_SESSION_ID_KEYS);
}

export function readTerminalControlSessionId(event: NexusOperationEvent): string | null {
  return readInputString(event.input_preview, SESSION_ID_KEYS)
    ?? readResultString(event.result_preview, RESULT_SESSION_ID_KEYS)
    ?? readTarget(event, "KillShell");
}

function readInputString(
  input: Record<string, unknown> | null | undefined,
  keys: readonly string[],
): string | null {
  if (!input) {
    return null;
  }
  for (const key of keys) {
    const value = scalarString(input[key]);
    if (value) {
      return value;
    }
  }
  return null;
}

function readResultString(value: unknown, keys: readonly string[]): string | null {
  if (typeof value === "string") {
    const parsed = parseStructuredString(value);
    return parsed == null ? null : readResultString(parsed, keys);
  }
  if (!value || typeof value !== "object") {
    return null;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const result = readResultString(item, keys);
      if (result) {
        return result;
      }
    }
    return null;
  }

  const record = value as Record<string, unknown>;
  for (const key of keys) {
    const result = scalarString(record[key]);
    if (result) {
      return result;
    }
  }
  return readResultString(record.content, keys)
    ?? readResultString(record.result, keys)
    ?? readResultString(record.data, keys);
}

function parseStructuredString(value: string): unknown | null {
  const trimmed = value.trim();
  if (!(trimmed.startsWith("{") || trimmed.startsWith("["))) {
    return null;
  }
  try {
    return JSON.parse(trimmed);
  } catch {
    return null;
  }
}

function scalarString(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return null;
}

function normalizeTerminalToolName(toolName: string | null | undefined): string {
  return (toolName ?? "")
    .trim()
    .toLowerCase()
    .replace(/^functions\./, "");
}

function readTarget(event: NexusOperationEvent, ignoredTarget: string): string | null {
  const target = event.target?.trim();
  return target && target !== ignoredTarget ? target : null;
}

function compareEvents(left: NexusOperationEvent, right: NexusOperationEvent): number {
  return (left.started_at ?? left.updated_at) - (right.started_at ?? right.updated_at);
}

function dedupeEvents(events: NexusOperationEvent[]): NexusOperationEvent[] {
  return [...new Map(events.map((event) => [event.id, event])).values()];
}
