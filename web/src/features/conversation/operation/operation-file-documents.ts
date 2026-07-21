/**
 * INPUT: One operation event, its round events, and workspace activity snapshots.
 * OUTPUT: Canonical document plans for real file opens; search expressions remain Files results.
 * POS: File identity boundary; absolute tool paths and relative workspace paths converge here.
 */
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
} from "./operation-types";
import { getWorkspaceFilePreviewKind } from "../shared/editor/workspace-file-preview-kind";
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
  const active_evidence_target = extract_structured_file_targets(event).at(0) ?? null;
  const latest_file_event = file_events.at(-1) ?? (active_evidence_target ? event : undefined);
  const project_active_event = is_file_document_event(event) || Boolean(active_evidence_target);
  const latest_file_target = latest_workspace_item?.path
    ?? (latest_file_event ? extract_file_targets_from_event(latest_file_event).at(0) : null)
    ?? active_evidence_target
    ?? (
      (event.surface === "workspace" || event.surface === "editor") &&
      event.target &&
      is_local_file_reference(event.target)
        ? event.target
        : null
    );
  const latest_file_preview = latest_workspace_item?.live_content ?? (
    latest_file_event && is_file_document_event(latest_file_event)
      ? latest_file_event.result_preview
        ?? latest_file_event.input_preview
        ?? latest_file_event.summary
      : structured_file_preview(latest_file_event, latest_file_target)
        ?? latest_file_event?.summary
        ?? null
  );

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
  const preview_kind = getWorkspaceFilePreviewKind(target);
  if (preview_kind === "markdown") return "markdown_reader";
  if (preview_kind === "document") return "word_reader";
  if (preview_kind === "presentation") return "presentation";
  if (preview_kind === "pdf") return "pdf_reader";
  if (preview_kind === "spreadsheet") return "spreadsheet";
  if (preview_kind === "image") return "image_viewer";
  if (preview_kind === "text" || preview_kind === "html") return "code_editor";
  return "file_preview";
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
      .flatMap((item) => extract_file_targets_from_event(item))
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

function extract_file_targets_from_event(event: NexusOperationEvent): string[] {
  const input = event.input_preview;
  const targets = extract_structured_file_targets(event);
  if (is_file_document_event(event)) {
    for (const key of ["file_path", "filePath", "notebook_path", "path", "source"] as const) {
      if (
        typeof input?.[key] === "string" &&
        input[key].trim() &&
        is_local_file_reference(input[key])
      ) {
        targets.push(input[key]);
      }
    }
    for (const collection_key of ["edits", "creates", "files"] as const) {
      const collection = input?.[collection_key];
      if (!Array.isArray(collection)) {
        continue;
      }
      for (const item of collection) {
        if (!item || typeof item !== "object" || Array.isArray(item)) {
          continue;
        }
        const record = item as Record<string, unknown>;
        const path = record.path ?? record.file_path ?? record.filePath;
        if (typeof path === "string" && path.trim() && is_local_file_reference(path)) {
          targets.push(path);
        }
      }
    }
    if (
      event.target &&
      event.target !== event.tool_name &&
      is_local_file_reference(event.target)
    ) {
      targets.push(event.target);
    }
  }
  return Array.from(new Set(targets.map(normalize_operation_file_target).filter(Boolean)));
}

function extract_structured_file_targets(event: NexusOperationEvent): string[] {
  return (event.evidence ?? []).flatMap((item) => {
    if (item.type !== "file" && item.type !== "artifact") {
      return [];
    }
    const candidate = item.value?.trim() || (
      looks_like_file_label(item.label) ? item.label.trim() : ""
    );
    return candidate && looks_like_file_label(candidate) && is_local_file_reference(candidate)
      ? [candidate]
      : [];
  });
}

function structured_file_preview(
  event: NexusOperationEvent | undefined,
  target: string | null | undefined,
): unknown {
  if (!event) {
    return null;
  }
  return (event.evidence ?? []).find((item) => (
    (item.type === "file" || item.type === "artifact") && (
      !target ||
      Boolean(item.value && operationWorkspaceTargetsMatch(item.value, target)) ||
      operationWorkspaceTargetsMatch(item.label, target)
    )
  ))?.preview ?? null;
}

function looks_like_file_label(value: string): boolean {
  const normalized = value.trim();
  return normalized.startsWith(".")
    || normalized.includes("/")
    || normalized.includes("\\")
    || /\.[a-z0-9]{1,12}$/i.test(normalized)
    || /^(?:dockerfile|gemfile|makefile|procfile|rakefile|readme|license)$/i.test(normalized);
}

function is_local_file_reference(value: string): boolean {
  const normalized = value.trim();
  if (/^file:\/\//i.test(normalized)) {
    return true;
  }
  return !/^(?:(?:[a-z][a-z0-9+.-]*:)?\/\/|data:|blob:)/i.test(normalized);
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
    const file_targets = extract_file_targets_from_event(file_event);
    if (file_targets.length === 0) {
      return;
    }
    file_targets.forEach((file_target) => {
      const canonical_target = resolveOperationWorkspaceTarget(file_target, workspace_items);
      const document_key = find_matching_target_key(documents, canonical_target) ?? canonical_target;
      const event_key = find_matching_target_key(file_events_by_target, canonical_target) ?? document_key;
      const workspace_item = workspace_items.find((item) => (
        operationWorkspaceTargetsMatch(canonical_target, item.path)
      )) ?? null;
      const events_for_target = file_events_by_target.get(event_key) ?? [];
      events_for_target.push(file_event);
      file_events_by_target.set(event_key, events_for_target);
      documents.set(document_key, {
        event: file_event,
        target: canonical_target,
        workspace_item,
        preview: file_event.result_preview ?? file_event.input_preview ?? file_event.summary,
        related_events: events_for_target,
      });
    });
  });

  workspace_items.forEach((workspace_item) => {
    if (!workspace_item.path) {
      return;
    }
    const document_key = find_matching_target_key(documents, workspace_item.path) ?? workspace_item.path;
    const event_key = find_matching_target_key(file_events_by_target, workspace_item.path);
    const existing = documents.get(document_key);
    const related_events = event_key ? file_events_by_target.get(event_key) ?? [] : [];
    const document_event = existing?.event
      ?? related_events.at(-1)
      ?? (project_active_event && workspace_item.path === latest_file_target ? event : null);
    if (!document_event) {
      return;
    }
    documents.set(document_key, {
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

  const latest_document_key = latest_file_target
    ? find_matching_target_key(documents, latest_file_target)
    : null;
  if (project_active_event && latest_file_target && !latest_document_key) {
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

function find_matching_target_key<T>(map: Map<string, T>, target: string): string | null {
  return Array.from(map.keys()).find((key) => operationWorkspaceTargetsMatch(key, target)) ?? null;
}

function is_file_document_event(event: NexusOperationEvent): boolean {
  return event.kind === "workspace_read"
    || event.kind === "workspace_edit"
    || event.kind === "artifact_update"
    || event.surface === "editor";
}
