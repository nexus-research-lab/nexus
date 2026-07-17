/**
 * INPUT: One operation event, its round events, and workspace activity snapshots.
 * OUTPUT: Canonical document plans for real file opens; search expressions remain Files results.
 * POS: File identity boundary; absolute tool paths and relative workspace paths converge here.
 */
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";
import type { StageWindowKind } from "./operation-desktop-types";

export interface OperationFileDocumentPlan {
  event: NexusOperationEvent;
  target: string;
  workspace_item: NexusOperationSnapshot["workspace_events"][number] | null;
  preview: unknown;
  related_events: NexusOperationEvent[];
}

interface OperationFileContext {
  file_documents: OperationFileDocumentPlan[];
  latest_file_event: NexusOperationEvent | undefined;
  latest_file_preview: unknown;
  latest_file_target: string | null | undefined;
  latest_workspace_item: NexusOperationSnapshot["workspace_events"][number] | null;
  workspace_items: NexusOperationSnapshot["workspace_events"];
}

export function collectOperationFileContext(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
  round_events: NexusOperationEvent[],
): OperationFileContext {
  const file_events = round_events.filter(is_file_document_event);
  const workspace_items = collect_round_workspace_items(event, snapshot, round_events);
  const latest_workspace_item = find_latest_workspace_item(event, snapshot, workspace_items);
  const latest_file_event = file_events.at(-1);
  const latest_result_file_target = can_project_result_file(event)
    ? extract_file_target_from_event(event)
    : null;
  const project_active_event = is_file_document_event(event) || Boolean(latest_result_file_target);
  const latest_file_target = latest_workspace_item?.path
    ?? latest_file_event?.target
    ?? latest_result_file_target
    ?? (event.surface === "workspace" || event.surface === "editor" ? event.target : null);
  const latest_file_preview = latest_workspace_item?.live_content
    ?? latest_file_event?.result_preview
    ?? latest_file_event?.input_preview
    ?? latest_file_event?.summary
    ?? null;

  return {
    file_documents: collect_file_documents({
      event,
      file_events,
      latest_file_preview,
      latest_file_target,
      latest_workspace_item,
      project_active_event,
      round_events,
      workspace_items,
    }),
    latest_file_event,
    latest_file_preview,
    latest_file_target,
    latest_workspace_item,
    workspace_items,
  };
}

export function windowKindForFileTarget(
  target?: string | null,
  fallback: StageWindowKind = "code_editor",
): StageWindowKind {
  if (!target) {
    return fallback;
  }
  const normalized = target.toLowerCase().split("?")[0] ?? "";
  const extension = normalized.includes(".")
    ? normalized.slice(normalized.lastIndexOf(".") + 1)
    : "";
  if (["md", "mdx", "markdown"].includes(extension)) {
    return "markdown_reader";
  }
  if (["doc", "docx", "rtf", "odt"].includes(extension)) {
    return "word_reader";
  }
  if (["ppt", "pptx", "odp"].includes(extension)) {
    return "presentation";
  }
  if (extension === "pdf") {
    return "pdf_reader";
  }
  if (["csv", "tsv", "xls", "xlsx", "ods"].includes(extension)) {
    return "spreadsheet";
  }
  if (["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif", "tif", "tiff"].includes(extension)) {
    return "image_viewer";
  }
  return fallback;
}

export function fallbackWindowKindForFileEvent(event: NexusOperationEvent): StageWindowKind {
  if (
    event.kind === "workspace_read" ||
    event.kind === "workspace_edit" ||
    event.kind === "artifact_update" ||
    event.surface === "editor"
  ) {
    return "code_editor";
  }
  return "finder";
}

export function resolveOperationWorkspaceTarget(
  target: string,
  workspace_items: NexusOperationSnapshot["workspace_events"],
): string {
  const matched_item = workspace_items.find((item) => (
    operationWorkspaceTargetsMatch(target, item.path)
  ));
  return matched_item?.path ?? normalize_operation_file_target(target);
}

export function operationWorkspaceTargetsMatch(left: string, right: string): boolean {
  const normalized_left = normalize_operation_file_target(left);
  const normalized_right = normalize_operation_file_target(right);
  return normalized_left === normalized_right
    || normalized_left.endsWith(`/${normalized_right}`)
    || normalized_right.endsWith(`/${normalized_left}`);
}

function normalize_operation_file_target(target: string): string {
  return target
    .trim()
    .replace(/^file:\/\//i, "")
    .split(/[?#]/, 1)[0]
    ?.replace(/\\/g, "/")
    .replace(/^\.\//, "")
    .replace(/\/{2,}/g, "/") ?? "";
}

function find_latest_workspace_item(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
  workspace_items?: NexusOperationSnapshot["workspace_events"],
) {
  const items = workspace_items ?? snapshot?.workspace_events ?? [];
  if (!items.length) {
    return null;
  }
  const target_item = event.target
    ? latest_workspace_item(items.filter((item) => item.path === event.target))
    : null;
  return target_item ?? latest_workspace_item(items);
}

function latest_workspace_item(
  items: NexusOperationSnapshot["workspace_events"],
): NexusOperationSnapshot["workspace_events"][number] | null {
  return items
    .slice()
    .sort((left, right) => right.updated_at - left.updated_at)
    .at(0) ?? null;
}

function collect_round_workspace_items(
  event: NexusOperationEvent,
  snapshot: NexusOperationSnapshot | null,
  round_events: NexusOperationEvent[],
): NexusOperationSnapshot["workspace_events"] {
  const workspace_items = snapshot?.workspace_events ?? [];
  if (!workspace_items.length) {
    return [];
  }

  const round_tool_use_ids = new Set(
    round_events
      .map((item) => item.tool_use_id)
      .filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id)),
  );
  const round_targets = new Set(
    round_events
      .flatMap((item) => [item.target, extract_file_target_from_event(item)])
      .filter((target): target is string => Boolean(target)),
  );

  const scoped_items = workspace_items.filter((item) => (
    Boolean(item.tool_use_id && round_tool_use_ids.has(item.tool_use_id)) ||
    round_targets.has(item.path)
  ));

  if (scoped_items.length > 0) {
    return scoped_items.slice(0, 8);
  }

  const event_target_item = event.target
    ? workspace_items.find((item) => item.path === event.target)
    : null;
  return (event_target_item ? [event_target_item] : []).slice(0, 8);
}

function extract_file_target_from_event(event: NexusOperationEvent): string | null {
  return first_local_file_path(event.result_preview)
    ?? first_local_file_path(event.evidence)
    ?? null;
}

function first_local_file_path(value: unknown): string | null {
  if (value == null) {
    return null;
  }
  if (typeof value === "string") {
    return extract_local_file_path(value);
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      const path = first_local_file_path(item);
      if (path) {
        return path;
      }
    }
    return null;
  }
  if (typeof value === "object") {
    for (const item of Object.values(value)) {
      const path = first_local_file_path(item);
      if (path) {
        return path;
      }
    }
  }
  return null;
}

function extract_local_file_path(text: string): string | null {
  if (/^(https?:|data:|blob:)/i.test(text.trim())) {
    return null;
  }
  const match = text.match(/(?:\.{0,2}\/)?[\w@./-]+\.(?:png|jpe?g|gif|webp|svg|pdf|mdx?|markdown|csv|tsv|xlsx?|docx?|rtf|odt)\b/i);
  return match?.[0] ?? null;
}

function collect_file_documents({
  event,
  file_events,
  latest_file_preview,
  latest_file_target,
  latest_workspace_item,
  project_active_event,
  round_events,
  workspace_items,
}: {
  event: NexusOperationEvent;
  file_events: NexusOperationEvent[];
  latest_file_preview: unknown;
  latest_file_target?: string | null;
  latest_workspace_item: NexusOperationSnapshot["workspace_events"][number] | null;
  project_active_event: boolean;
  round_events: NexusOperationEvent[];
  workspace_items: NexusOperationSnapshot["workspace_events"];
}): OperationFileDocumentPlan[] {
  const documents = new Map<string, OperationFileDocumentPlan>();
  const file_events_by_target = new Map<string, NexusOperationEvent[]>();

  file_events.forEach((file_event) => {
    if (!file_event.target) {
      return;
    }
    const canonical_target = resolveOperationWorkspaceTarget(file_event.target, workspace_items);
    const workspace_item = workspace_items.find((item) => (
      operationWorkspaceTargetsMatch(canonical_target, item.path)
    )) ?? null;
    const events_for_target = file_events_by_target.get(canonical_target) ?? [];
    events_for_target.push(file_event);
    file_events_by_target.set(canonical_target, events_for_target);
    documents.set(canonical_target, {
      event: file_event,
      target: canonical_target,
      workspace_item,
      preview: file_event.result_preview ?? file_event.input_preview ?? file_event.summary,
      related_events: events_for_target,
    });
  });

  workspace_items.forEach((workspace_item) => {
    if (!workspace_item.path) {
      return;
    }
    const existing = documents.get(workspace_item.path);
    const related_events = file_events_by_target.get(workspace_item.path) ?? [];
    const document_event = existing?.event
      ?? related_events.at(-1)
      ?? (project_active_event && workspace_item.path === latest_file_target ? event : null);
    if (!document_event) {
      return;
    }
    documents.set(workspace_item.path, {
      event: document_event,
      target: workspace_item.path,
      workspace_item,
      preview: workspace_item.live_content
        ?? existing?.preview
        ?? document_event.result_preview
        ?? document_event.input_preview
        ?? document_event.summary,
      related_events: related_events.length ? related_events : [document_event],
    });
  });

  if (project_active_event && latest_file_target && !documents.has(latest_file_target)) {
    documents.set(latest_file_target, {
      event,
      target: latest_file_target,
      workspace_item: latest_workspace_item,
      preview: latest_file_preview,
      related_events: round_events.filter((item) => (
        item.target && operationWorkspaceTargetsMatch(item.target, latest_file_target)
      )),
    });
  }

  return Array.from(documents.values())
    .sort((left, right) => right.event.updated_at - left.event.updated_at)
    .slice(0, 4)
    .reverse();
}

function is_file_document_event(event: NexusOperationEvent): boolean {
  return event.kind === "workspace_read"
    || event.kind === "workspace_edit"
    || event.kind === "artifact_update"
    || event.surface === "editor";
}

function can_project_result_file(event: NexusOperationEvent): boolean {
  return event.kind !== "workspace_search" && event.kind !== "workspace_inspect";
}
