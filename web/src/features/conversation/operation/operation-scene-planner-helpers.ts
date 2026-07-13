import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";

export function collectRoundEvents(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
): NexusOperationEvent[] {
  const events = snapshot?.events.filter((item) => item.round_id === event.round_id) ?? [];
  const merged = events.some((item) => item.id === event.id) ? events : [...events, event];
  const sorted = merged
    .sort((left, right) => left.updated_at - right.updated_at)
    .slice(-12);
  const active_index = sorted.findIndex((item) => item.id === event.id);
  if (active_index < 0) {
    return sorted;
  }
  return sorted.slice(0, active_index + 1);
}

export function normalizeWindowId(value: string): string {
  return value.replace(/[^a-zA-Z0-9._-]+/g, "-").slice(-80);
}

export function basename(value?: string | null): string {
  if (!value) {
    return "preview";
  }
  return value.split("/").filter(Boolean).at(-1) ?? value;
}

export function looksLikeUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

export function previewLines(value: unknown, max_lines: number): string[] {
  if (value == null) {
    return [];
  }
  const text = typeof value === "string" ? value : safeJsonStringify(value);
  return text
    .split(/\r?\n/)
    .map((line) => line.trimEnd())
    .filter(Boolean)
    .slice(0, max_lines);
}

export function readInputString(
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

function safeJsonStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
