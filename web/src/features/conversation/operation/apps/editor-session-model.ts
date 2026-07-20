/**
 * INPUT: File-scoped tool events plus live workspace status and diff statistics.
 * OUTPUT: A truthful Editor session with action history, source focus, and exact edit fragments.
 * POS: Pure semantic model for the Operation Stage Editor app.
 */
import { operationWorkspaceTargetsMatch } from "../operation-file-documents";
import type { NexusOperationEvent, OperationPhase } from "../operation-types";
import { resolveOperationToolProfile } from "../operation-tool-catalog";

export type EditorActionKind =
  | "read"
  | "write"
  | "edit"
  | "multi_edit"
  | "notebook_edit"
  | "sync"
  | "preview";

export interface EditorLineRange {
  end: number | null;
  start: number;
}

export interface EditorDiffStats {
  additions: number;
  changed_lines?: number;
  deletions: number;
}

export interface EditorChangeFragment {
  after: string;
  before: string;
  id: string;
  replaceAll: boolean;
}

export interface EditorActionView {
  changeCount: number;
  contentCharacters: number | null;
  contentLines: number | null;
  id: string;
  kind: EditorActionKind;
  label: string;
  lineRange: EditorLineRange | null;
  phase: OperationPhase;
  statusLabel: string;
  toolName: string | null;
}

export interface EditorSourceFocus {
  endLine?: number | null;
  snippets?: string[];
  startLine?: number | null;
  tone: "read" | "write" | "edit" | "error";
}

export interface EditorSessionView {
  activeAction: EditorActionView;
  changes: EditorChangeFragment[];
  detailLabel: string | null;
  diffStats: EditorDiffStats | null;
  history: EditorActionView[];
  sourceFocus: EditorSourceFocus | null;
}

const ACTION_LABELS: Record<EditorActionKind, string> = {
  edit: "修改文件",
  multi_edit: "批量修改",
  notebook_edit: "修改 Notebook",
  preview: "查看文件",
  read: "读取文件",
  sync: "同步文件",
  write: "创建文件",
};

const PHASE_LABELS: Record<OperationPhase, string> = {
  cancelled: "已中断",
  done: "已完成",
  error: "执行失败",
  queued: "等待执行",
  running: "正在执行",
  waiting: "等待确认",
};

export function buildEditorSessionView({
  diffStats,
  event,
  liveStatus,
  path,
  relatedEvents,
}: {
  diffStats?: EditorDiffStats | null;
  event: NexusOperationEvent;
  liveStatus?: "deleted" | "idle" | "updated" | "writing" | null;
  path: string;
  relatedEvents: NexusOperationEvent[];
}): EditorSessionView {
  const scoped_events = collect_editor_events(event, relatedEvents, path);
  const action_events = scoped_events.filter((item) => item.tool_name !== "workspace_event");
  const source_events = action_events.length > 0 ? action_events : scoped_events;
  const history = source_events.map((item) => build_editor_action(item, path));
  const fallback_action = build_preview_action(event);
  const base_active_action = history.at(-1) ?? fallback_action;
  const activeAction = liveStatus === "writing"
    ? { ...base_active_action, phase: "running" as const, statusLabel: "正在写入" }
    : base_active_action;
  const active_event = source_events.at(-1) ?? event;
  const changes = extract_editor_changes(active_event, path);
  const resolved_diff_stats = diffStats ?? null;

  return {
    activeAction,
    changes,
    detailLabel: build_detail_label(activeAction, resolved_diff_stats),
    diffStats: resolved_diff_stats,
    history,
    sourceFocus: build_source_focus(activeAction, active_event, changes),
  };
}

function collect_editor_events(
  event: NexusOperationEvent,
  related_events: NexusOperationEvent[],
  path: string,
): NexusOperationEvent[] {
  const by_identity = new Map<string, NexusOperationEvent>();
  for (const candidate of [...related_events, event]) {
    const action = resolve_editor_action_kind(candidate, path);
    if (!action || !event_matches_path(candidate, path)) {
      continue;
    }
    const identity = candidate.tool_use_id
      ? `${candidate.tool_use_id}:${candidate.tool_name ?? action}`
      : candidate.id;
    const previous = by_identity.get(identity);
    if (!previous || compare_editor_events(previous, candidate) <= 0) {
      by_identity.set(identity, candidate);
    }
  }
  return Array.from(by_identity.values())
    .sort(compare_editor_events)
    .slice(-8);
}

function build_editor_action(event: NexusOperationEvent, path: string): EditorActionView {
  const kind = resolve_editor_action_kind(event, path) ?? "preview";
  const content = read_write_content(event.input_preview, path);
  const changes = extract_editor_changes(event, path);
  return {
    changeCount: changes.length,
    contentCharacters: content == null ? null : content.length,
    contentLines: content == null ? null : count_lines(content),
    id: event.id,
    kind,
    label: ACTION_LABELS[kind],
    lineRange: kind === "read" ? read_line_range(event.input_preview) : null,
    phase: event.phase,
    statusLabel: PHASE_LABELS[event.phase],
    toolName: event.tool_name ?? null,
  };
}

function build_preview_action(event: NexusOperationEvent): EditorActionView {
  return {
    changeCount: 0,
    contentCharacters: null,
    contentLines: null,
    id: event.id,
    kind: "preview",
    label: ACTION_LABELS.preview,
    lineRange: null,
    phase: event.phase,
    statusLabel: PHASE_LABELS[event.phase],
    toolName: event.tool_name ?? null,
  };
}

function resolve_editor_action_kind(
  event: NexusOperationEvent,
  path?: string,
): EditorActionKind | null {
  if (event.tool_name === "workspace_event") {
    return "sync";
  }
  if (event.tool_name === "NotebookEdit" || event.tool_name === "notebook.edit") {
    return "notebook_edit";
  }
  if (event.tool_name === "MultiEdit") {
    return "multi_edit";
  }
  if (event.tool_name === "patch.apply" && has_patch_create(event.input_preview, path)) {
    return "write";
  }
  const profile = resolveOperationToolProfile(event.tool_name, event.kind, event.surface);
  if (profile.action === "read") {
    return "read";
  }
  if (profile.action === "create") {
    return "write";
  }
  if (profile.action === "edit") {
    return "edit";
  }
  return null;
}

function build_source_focus(
  action: EditorActionView,
  event: NexusOperationEvent,
  changes: EditorChangeFragment[],
): EditorSourceFocus | null {
  if (action.kind === "read" && action.lineRange) {
    return {
      endLine: action.lineRange.end,
      startLine: action.lineRange.start,
      tone: event.phase === "error" ? "error" : "read",
    };
  }
  if (action.kind === "read") {
    return null;
  }
  if (action.kind === "write") {
    return {
      endLine: action.contentLines,
      startLine: action.contentLines ? 1 : null,
      tone: event.phase === "error" ? "error" : "write",
    };
  }
  if (["edit", "multi_edit", "notebook_edit"].includes(action.kind)) {
    const use_before = event.phase === "error";
    return {
      snippets: changes
        .map((change) => use_before ? change.before : change.after)
        .filter(Boolean),
      tone: use_before ? "error" : "edit",
    };
  }
  return null;
}

function build_detail_label(
  action: EditorActionView,
  diff_stats: EditorDiffStats | null,
): string | null {
  if (action.lineRange) {
    return action.lineRange.end
      ? `L${action.lineRange.start}-L${action.lineRange.end}`
      : `从 L${action.lineRange.start}`;
  }
  if (action.kind === "read") {
    return "全文";
  }
  if (action.changeCount > 0) {
    const diff = diff_stats ? ` · +${diff_stats.additions} -${diff_stats.deletions}` : "";
    return `${action.changeCount} 处修改${diff}`;
  }
  if (action.contentCharacters != null) {
    return `${action.contentLines ?? 0} 行 · ${action.contentCharacters} 字符`;
  }
  if (diff_stats) {
    return `+${diff_stats.additions} -${diff_stats.deletions}`;
  }
  return null;
}

function extract_editor_changes(
  event: NexusOperationEvent,
  path: string,
): EditorChangeFragment[] {
  const input = event.input_preview;
  if (!input) {
    return [];
  }
  const direct_before = read_string(input, ["old_string", "oldString", "old_text", "oldText"]);
  const direct_after = read_string(input, ["new_string", "newString", "new_text", "newText", "new_source"]);
  if (direct_before != null || direct_after != null) {
    return [{
      after: direct_after ?? "",
      before: direct_before ?? "",
      id: `${event.id}:change:0`,
      replaceAll: read_boolean(input, ["replace_all", "replaceAll"]),
    }];
  }
  const edits = Array.isArray(input.edits) ? input.edits : [];
  return edits.flatMap((value, index) => {
    const item = as_record(value);
    if (!item || !record_matches_path(item, path)) {
      return [];
    }
    const before = read_string(item, ["old_string", "oldString", "old_text", "oldText"]);
    const after = read_string(item, ["new_string", "newString", "new_text", "newText"]);
    if (before == null && after == null) {
      return [];
    }
    return [{
      after: after ?? "",
      before: before ?? "",
      id: `${event.id}:change:${index}`,
      replaceAll: read_boolean(item, ["replace_all", "replaceAll"]),
    }];
  });
}

function read_line_range(
  input?: Record<string, unknown> | null,
): EditorLineRange | null {
  const has_explicit_range = Boolean(input && [
    "offset",
    "start_line",
    "startLine",
    "limit",
    "end_line",
    "endLine",
  ].some((key) => input[key] != null));
  if (!has_explicit_range) {
    return null;
  }
  const start = Math.max(1, read_number(input, ["offset", "start_line", "startLine"]) ?? 1);
  const limit = read_number(input, ["limit"]);
  const explicit_end = read_number(input, ["end_line", "endLine"]);
  return {
    end: explicit_end != null
      ? Math.max(start, explicit_end)
      : limit != null
        ? start + Math.max(1, limit) - 1
        : null,
    start,
  };
}

function read_write_content(
  input: Record<string, unknown> | null | undefined,
  path: string,
): string | null {
  const direct = read_string(input, ["content"]);
  if (direct != null) {
    return direct;
  }
  const creates = Array.isArray(input?.creates) ? input.creates : [];
  for (const value of creates) {
    const item = as_record(value);
    if (item && record_matches_path(item, path)) {
      return read_string(item, ["content"]);
    }
  }
  return null;
}

function event_matches_path(event: NexusOperationEvent, path: string): boolean {
  return event_file_targets(event).some((target) => operationWorkspaceTargetsMatch(target, path));
}

function event_file_targets(event: NexusOperationEvent): string[] {
  const input = event.input_preview;
  const targets = [
    event.target,
    read_string(input, ["file_path", "filePath", "notebook_path", "path"]),
  ].filter((value): value is string => Boolean(value));
  for (const key of ["edits", "creates"] as const) {
    const items = Array.isArray(input?.[key]) ? input[key] : [];
    for (const value of items) {
      const item = as_record(value);
      const target = read_string(item, ["file_path", "filePath", "path"]);
      if (target) {
        targets.push(target);
      }
    }
  }
  return targets;
}

function record_matches_path(value: Record<string, unknown>, path: string): boolean {
  const target = read_string(value, ["file_path", "filePath", "path"]);
  return !target || !path || operationWorkspaceTargetsMatch(target, path);
}

function has_patch_create(
  input?: Record<string, unknown> | null,
  path?: string,
): boolean {
  const creates = Array.isArray(input?.creates) ? input.creates : [];
  return creates.some((value) => {
    const item = as_record(value);
    return Boolean(item && (!path || record_matches_path(item, path)));
  });
}

function read_string(
  input: Record<string, unknown> | null | undefined,
  keys: string[],
): string | null {
  if (!input) {
    return null;
  }
  for (const key of keys) {
    if (typeof input[key] === "string") {
      return input[key] as string;
    }
  }
  return null;
}

function read_boolean(input: Record<string, unknown>, keys: string[]): boolean {
  return keys.some((key) => input[key] === true);
}

function read_number(
  input: Record<string, unknown> | null | undefined,
  keys: string[],
): number | null {
  if (!input) {
    return null;
  }
  for (const key of keys) {
    const value = input[key];
    if (typeof value === "number" && Number.isFinite(value)) {
      return Math.floor(value);
    }
    if (typeof value === "string" && /^\d+$/.test(value.trim())) {
      return Number(value);
    }
  }
  return null;
}

function as_record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function count_lines(value: string): number {
  return value ? value.split("\n").length : 0;
}

function compare_editor_events(left: NexusOperationEvent, right: NexusOperationEvent): number {
  return (left.updated_at - right.updated_at) || left.id.localeCompare(right.id);
}
