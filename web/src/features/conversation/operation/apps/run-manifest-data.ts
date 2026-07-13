import {
  AlertTriangle,
  ClipboardList,
  FileSpreadsheet,
  FileText,
  Globe2,
  ImageIcon,
  Terminal,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
  OperationPhase,
} from "../operation-types";
import { resolve_operation_event_output_label } from "../operation-event-io";

export const PHASE_LABEL: Record<OperationPhase, string> = {
  queued: "排队中",
  running: "执行中",
  waiting: "等待确认",
  done: "已完成",
  error: "失败",
  cancelled: "已中断",
};

export interface ManifestArtifact {
  id: string;
  label: string;
  value: string;
  type: OperationEvidence["type"];
}

export function collect_manifest_artifacts(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
  evidence: OperationEvidence[],
): ManifestArtifact[] {
  const artifacts = new Map<string, ManifestArtifact>();
  const add_artifact = (artifact: ManifestArtifact) => {
    const key = normalize_manifest_artifact_key(artifact.value);
    if (artifacts.has(key)) {
      return;
    }
    artifacts.set(key, artifact);
  };

  for (const item of events) {
    if (
      !item.target ||
      item.kind === "round_summary" ||
      item.surface === "terminal" ||
      item.surface === "conversation"
    ) {
      continue;
    }
    if ((item.surface === "workspace" || item.surface === "editor") && !looks_like_file_artifact(item.target)) {
      continue;
    }
    add_artifact({
      id: `event:${item.target}`,
      label: item.surface === "web" ? "Navi 页面" : item.surface === "task" ? "活动记录" : "工作区文件",
      value: item.target,
      type: item.surface === "web" ? "url" : item.surface === "task" ? "task" : "file",
    });
  }

  for (const item of collect_manifest_workspace_artifacts(event, events, snapshot)) {
    add_artifact({
      id: `workspace:${item.path}`,
      label: item.status === "deleted" ? "删除记录" : "文件快照",
      value: item.path,
      type: "file",
    });
  }

  for (const item of evidence) {
    if (!item.value || (item.type !== "file" && item.type !== "diff" && item.type !== "url" && item.type !== "artifact")) {
      continue;
    }
    if ((item.type === "file" || item.type === "diff" || item.type === "artifact") && !looks_like_file_artifact(item.value)) {
      continue;
    }
    add_artifact({
      id: `evidence:${item.type}:${item.value}`,
      label: item.label || evidence_type_label(item.type),
      value: item.value,
      type: item.type,
    });
  }

  if (event.target && artifacts.size === 0) {
    add_artifact({
      id: `event:${event.target}`,
      label: "当前目标",
      value: event.target,
      type: "status",
    });
  }

  return [...artifacts.values()].slice(0, 8);
}

export function format_manifest_duration(events: NexusOperationEvent[]): string {
  const started = events
    .map((item) => item.started_at ?? item.updated_at)
    .filter((value): value is number => Number.isFinite(value));
  const ended = events
    .map((item) => item.ended_at ?? item.updated_at)
    .filter((value): value is number => Number.isFinite(value));
  if (!started.length || !ended.length) {
    return "--";
  }
  const duration_seconds = Math.max(0, Math.round((Math.max(...ended) - Math.min(...started)) / 1000));
  if (duration_seconds < 60) {
    return `${duration_seconds}s`;
  }
  const minutes = Math.floor(duration_seconds / 60);
  const seconds = duration_seconds % 60;
  return `${minutes}m ${seconds}s`;
}

export function extract_manifest_result_text(event: NexusOperationEvent): string | null {
  if (typeof event.result_preview === "string" && event.result_preview.trim()) {
    return event.result_preview.trim();
  }
  if (event.result_preview && typeof event.result_preview === "object") {
    const record = event.result_preview as Record<string, unknown>;
    for (const key of ["result", "content", "message", "error"]) {
      const value = record[key];
      if (typeof value === "string" && value.trim()) {
        return value.trim();
      }
    }
  }
  return null;
}

export function extract_manifest_event_output(event: NexusOperationEvent): string | null {
  return resolve_operation_event_output_label(event);
}

export function icon_for_manifest_artifact(type: OperationEvidence["type"], value: string): LucideIcon {
  if (type === "terminal") {
    return Terminal;
  }
  if (type === "url") {
    return Globe2;
  }
  if (type === "task") {
    return ClipboardList;
  }
  if (type === "error") {
    return AlertTriangle;
  }
  if (type === "file" || type === "diff" || type === "artifact") {
    return icon_for_file_path(value);
  }
  return FileText;
}

function collect_manifest_workspace_artifacts(
  event: NexusOperationEvent,
  events: NexusOperationEvent[],
  snapshot: NexusOperationSnapshot | null,
): NexusOperationSnapshot["workspace_events"] {
  const workspace_items = snapshot?.workspace_events ?? [];
  if (!workspace_items.length) {
    return [];
  }

  const round_events = events.length
    ? events
    : snapshot?.events.filter((item) => item.round_id === event.round_id) ?? [event];
  const round_tool_use_ids = new Set(
    round_events
      .map((item) => item.tool_use_id)
      .filter((tool_use_id): tool_use_id is string => Boolean(tool_use_id)),
  );
  const round_targets = new Set(
    round_events
      .map((item) => item.target)
      .filter((target): target is string => Boolean(target)),
  );

  return workspace_items.filter((item) => (
    Boolean(item.tool_use_id && round_tool_use_ids.has(item.tool_use_id)) ||
    round_targets.has(item.path)
  ));
}

function normalize_manifest_artifact_key(value: string): string {
  return value.trim().replace(/\\+/g, "/").toLowerCase();
}

function looks_like_file_artifact(value: string): boolean {
  const normalized = value.trim();
  if (!normalized || normalized === "/" || normalized.endsWith("/")) {
    return false;
  }
  if (/^https?:\/\//i.test(normalized)) {
    return true;
  }
  const basename = normalized.split(/[\\/]/).filter(Boolean).at(-1) ?? normalized;
  return /\.[a-z0-9]{1,12}$/i.test(basename);
}

function evidence_type_label(type: OperationEvidence["type"]): string {
  if (type === "file" || type === "diff") {
    return "文件记录";
  }
  if (type === "terminal") {
    return "终端输出";
  }
  if (type === "url") {
    return "浏览器记录";
  }
  if (type === "artifact") {
    return "产物";
  }
  if (type === "task") {
    return "活动记录";
  }
  if (type === "permission") {
    return "安全记录";
  }
  if (type === "error") {
    return "错误报告";
  }
  return "日志记录";
}

function icon_for_file_path(value: string): LucideIcon {
  if (/\.(csv|xlsx?|ods)$/i.test(value)) {
    return FileSpreadsheet;
  }
  if (/\.(png|jpe?g|webp|gif|svg)$/i.test(value)) {
    return ImageIcon;
  }
  if (/\.(md|mdx|txt|docx?|pdf)$/i.test(value)) {
    return FileText;
  }
  return FileText;
}
