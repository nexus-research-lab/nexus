import type { NexusOperationEvent } from "./operation-types";
import {
  buildOperationInputRows,
  resolveOperationToolProfile,
} from "./operation-tool-catalog";

export interface OperationEventIOSummary {
  action_label: string;
  input_detail: string | null;
  input_label: string;
  output_label: string | null;
}

export function buildOperationEventIoSummary(event: NexusOperationEvent): OperationEventIOSummary {
  const profile = resolveOperationToolProfile(event.tool_name, event.kind, event.surface);
  const input_row = buildOperationInputRows(event.input_preview, profile.target_keys, 1)[0] ?? null;
  const fallback_input = event.target ?? event.summary ?? event.title;
  return {
    action_label: profile.action_label,
    input_detail: input_row ? `${input_row.label}: ${input_row.value}` : null,
    input_label: input_row ? `${input_row.label}: ${input_row.value}` : fallback_input,
    output_label: resolveOperationEventOutputLabel(event),
  };
}

export function resolveOperationEventOutputLabel(event: NexusOperationEvent): string | null {
  if (event.kind === "round_summary") {
    return event.summary ?? compact_operation_result(event.result_preview);
  }
  if (event.phase === "running") {
    return event.surface === "terminal" ? "等待 stdout/stderr" : "等待工具结果";
  }
  if (event.phase === "waiting") {
    return "等待确认";
  }

  const result_label = compact_operation_result(event.result_preview);
  if (event.phase === "error") {
    return result_label ?? evidence_label(event) ?? event.summary ?? "异常证据";
  }
  if (event.surface === "terminal") {
    return result_label ?? evidence_label(event) ?? event.summary ?? "命令结果";
  }
  return evidence_label(event) ?? event.summary ?? result_label ?? null;
}

function evidence_label(event: NexusOperationEvent): string | null {
  const evidence = event.evidence?.find((item) => item.value || item.label);
  return evidence?.value ?? evidence?.label ?? null;
}

function compact_operation_result(value: unknown): string | null {
  if (value == null) {
    return null;
  }
  if (typeof value === "string") {
    return value.trim().split(/\r?\n/).find(Boolean)?.slice(0, 160) ?? null;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => compact_operation_result(item)).find(Boolean) ?? null;
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    for (const key of ["stderr", "stdout", "output", "content", "error", "message", "result", "text"]) {
      const item = compact_operation_result(record[key]);
      if (item) {
        return item;
      }
    }
  }
  try {
    return JSON.stringify(value).slice(0, 160);
  } catch {
    return String(value).slice(0, 160);
  }
}
