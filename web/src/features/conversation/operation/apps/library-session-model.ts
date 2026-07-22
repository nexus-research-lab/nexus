/**
 * INPUT: Skill/knowledge operation events from one Agent round.
 * OUTPUT: Stable Library entries and the entry the Agent most recently used.
 * POS: Pure Library app session model; React only owns user selection and search state.
 */
import type { NexusOperationEvent } from "../operation-types";

export interface LibraryEntry {
  content: string;
  description: string | null;
  event: NexusOperationEvent;
  id: string;
  name: string;
  phase: NexusOperationEvent["phase"];
  updated_at: number;
}

export interface LibrarySessionView {
  active_entry_id: string | null;
  entries: LibraryEntry[];
}

export function buildLibrarySessionView({
  event,
  relatedEvents,
}: {
  event: NexusOperationEvent;
  relatedEvents: NexusOperationEvent[];
}): LibrarySessionView {
  const source_events = unique_events([event, ...relatedEvents])
    .filter(is_library_event)
    .sort((left, right) => left.updated_at - right.updated_at);
  const entries_by_name = new Map<string, LibraryEntry>();

  for (const source_event of source_events) {
    const entry = build_library_entry(source_event);
    entries_by_name.set(normalize_library_identity(entry.name), entry);
  }

  const entries = Array.from(entries_by_name.values())
    .sort((left, right) => right.updated_at - left.updated_at);
  const active_name = is_library_event(event) ? read_library_name(event) : null;
  const active_entry = active_name
    ? entries.find((entry) => normalize_library_identity(entry.name) === normalize_library_identity(active_name))
    : entries[0];

  return {
    active_entry_id: active_entry?.id ?? null,
    entries,
  };
}

export function filterLibraryEntries(
  entries: LibraryEntry[],
  query: string,
): LibraryEntry[] {
  const normalized_query = query.trim().toLocaleLowerCase();
  if (!normalized_query) {
    return entries;
  }
  return entries.filter((entry) => (
    entry.name.toLocaleLowerCase().includes(normalized_query)
    || entry.description?.toLocaleLowerCase().includes(normalized_query)
    || entry.content.toLocaleLowerCase().includes(normalized_query)
  ));
}

function build_library_entry(event: NexusOperationEvent): LibraryEntry {
  const name = read_library_name(event);
  return {
    content: read_library_content(event),
    description: read_input_string(event.input_preview, ["description", "summary"])
      ?? event.summary?.trim()
      ?? null,
    event,
    id: event.id,
    name,
    phase: event.phase,
    updated_at: event.updated_at,
  };
}

function read_library_name(event: NexusOperationEvent): string {
  return read_input_string(event.input_preview, ["skill_name", "name"])
    ?? meaningful_label(event.target, event.tool_name)
    ?? meaningful_label(event.title, event.tool_name)
    ?? event.tool_name
    ?? "上下文";
}

function read_library_content(event: NexusOperationEvent): string {
  return format_library_content(event.result_preview)
    ?? format_library_content(event.input_preview?.content)
    ?? format_library_content(event.input_preview?.instructions)
    ?? event.summary?.trim()
    ?? "当前条目尚未返回正文。";
}

function format_library_content(value: unknown): string | null {
  if (typeof value === "string") {
    return value.trim() || null;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    const sections = value.map(format_library_content).filter((item): item is string => Boolean(item));
    return sections.length ? sections.join("\n\n") : null;
  }
  if (!value || typeof value !== "object") {
    return null;
  }

  const record = value as Record<string, unknown>;
  for (const key of ["content", "markdown", "instructions", "text", "body", "result", "output"] as const) {
    const content = format_library_content(record[key]);
    if (content) {
      return content;
    }
  }

  try {
    return `\`\`\`json\n${JSON.stringify(record, null, 2)}\n\`\`\``;
  } catch {
    return null;
  }
}

function is_library_event(event: NexusOperationEvent): boolean {
  return event.surface === "knowledge"
    || event.tool_name === "Skill"
    || event.tool_name === "skill.invoke"
    || event.tool_name === "skill.use";
}

function meaningful_label(
  value: string | null | undefined,
  tool_name: string | null | undefined,
): string | null {
  const normalized = value?.trim();
  if (!normalized || normalized === tool_name || normalized.toLocaleLowerCase() === "skill") {
    return null;
  }
  return normalized;
}

function normalize_library_identity(value: string): string {
  return value.trim().toLocaleLowerCase();
}

function read_input_string(
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
  }
  return null;
}

function unique_events(events: NexusOperationEvent[]): NexusOperationEvent[] {
  const by_id = new Map<string, NexusOperationEvent>();
  for (const event of events) {
    const existing = by_id.get(event.id);
    if (!existing || event.updated_at >= existing.updated_at) {
      by_id.set(event.id, event);
    }
  }
  return Array.from(by_id.values());
}
