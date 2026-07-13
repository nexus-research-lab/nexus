import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { formatScheduledDatetime } from "../scheduled-formatters";
import { formatDuration } from "./scheduled-task-run-history-model";

export interface RunDiagnosticRow {
  breakAll?: boolean;
  label: string;
  value: string;
}

export interface RunOutputSection {
  content: string;
  label?: string;
  tone: "danger" | "default";
}

interface RunDiagnosticRowDefinition {
  breakAll?: boolean;
  label: string;
  value: (run: ScheduledTaskRunItem) => string | null;
}

interface RunOutputSectionDefinition {
  content: (run: ScheduledTaskRunItem) => string | null;
  label?: string;
  tone: RunOutputSection["tone"];
}

interface DiagnosticCopyFieldDefinition {
  label: string;
  value: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => string;
}

interface DiagnosticCopySectionDefinition {
  label: string;
  value: (run: ScheduledTaskRunItem) => string | null;
}

const formatDatetime = (value: number | null): string => (
  formatScheduledDatetime(value, { includeSeconds: true })
);

const optionalText = (value: string | null | undefined): string | null => value || null;

const optionalNumber = (value: number | null | undefined): string | null => (
  typeof value === "number" ? String(value) : null
);

const positiveNumber = (value: number | null | undefined): string | null => (
  value ? String(value) : null
);

const optionalDatetime = (value: number | null): string | null => (
  value ? formatDatetime(value) : null
);

function assistantText(run: ScheduledTaskRunItem): string | null {
  const content = run.assistant_text?.trim();
  return content && content !== (run.result_text ?? "").trim() ? run.assistant_text ?? null : null;
}

const RUN_DIAGNOSTIC_ROW_DEFINITIONS: readonly RunDiagnosticRowDefinition[] = [
  { breakAll: true, label: "Run", value: (run) => run.run_id },
  { label: "Trigger", value: (run) => optionalText(run.trigger_kind) },
  { label: "Messages", value: (run) => optionalNumber(run.message_count) },
  { breakAll: true, label: "Session", value: (run) => optionalText(run.session_key) },
  { breakAll: true, label: "Round", value: (run) => optionalText(run.round_id) },
  { breakAll: true, label: "Runtime", value: (run) => optionalText(run.session_id) },
  { breakAll: true, label: "Delivery", value: (run) => optionalText(run.delivery_to) },
  { label: "Delivered", value: (run) => optionalDatetime(run.delivered_at) },
  { label: "Delivery attempts", value: (run) => positiveNumber(run.delivery_attempts) },
  { label: "Next delivery retry", value: (run) => optionalDatetime(run.delivery_next_attempt_at) },
  { label: "Delivery dead letter", value: (run) => optionalDatetime(run.delivery_dead_letter_at) },
  { label: "Started", value: (run) => formatDatetime(run.started_at) },
  { label: "Finished", value: (run) => formatDatetime(run.finished_at) },
  { label: "Attempts", value: (run) => String(run.attempts) },
];

const RUN_OUTPUT_SECTION_DEFINITIONS: readonly RunOutputSectionDefinition[] = [
  { content: (run) => optionalText(run.error_message), tone: "danger" },
  {
    content: (run) => run.delivery_error ? `投递失败：${run.delivery_error}` : null,
    tone: "danger",
  },
  { content: (run) => optionalText(run.result_summary), tone: "default" },
  { content: (run) => optionalText(run.result_text), label: "运行输出", tone: "default" },
  { content: assistantText, label: "助手回复", tone: "default" },
];

const DIAGNOSTIC_COPY_FIELD_DEFINITIONS: readonly DiagnosticCopyFieldDefinition[] = [
  { label: "Task", value: (task) => task.name },
  { label: "Job ID", value: (task) => task.job_id },
  { label: "Agent ID", value: (task) => task.agent_id },
  { label: "Execution", value: (task) => task.execution_kind ?? "agent" },
  { label: "Run ID", value: (_task, run) => run.run_id },
  { label: "Status", value: (_task, run) => run.status },
  { label: "Delivery Status", value: (_task, run) => run.delivery_status || "" },
  { label: "Delivery Attempts", value: (_task, run) => String(run.delivery_attempts ?? 0) },
  { label: "Delivered At", value: (_task, run) => formatDatetime(run.delivered_at) },
  { label: "Delivery Next Attempt", value: (_task, run) => formatDatetime(run.delivery_next_attempt_at) },
  { label: "Delivery Dead Letter At", value: (_task, run) => formatDatetime(run.delivery_dead_letter_at) },
  { label: "Trigger", value: (_task, run) => run.trigger_kind || "" },
  { label: "Scheduled", value: (_task, run) => formatDatetime(run.scheduled_for) },
  { label: "Started", value: (_task, run) => formatDatetime(run.started_at) },
  { label: "Finished", value: (_task, run) => formatDatetime(run.finished_at) },
  {
    label: "Duration",
    value: (_task, run) => formatDuration(run.started_at, run.finished_at),
  },
  { label: "Attempts", value: (_task, run) => String(run.attempts) },
  { label: "Session", value: (_task, run) => run.session_key || "" },
  { label: "Round", value: (_task, run) => run.round_id || "" },
  { label: "Runtime", value: (_task, run) => run.session_id || "" },
  { label: "Artifact", value: (_task, run) => run.artifact_path || "" },
];

const DIAGNOSTIC_COPY_SECTION_DEFINITIONS: readonly DiagnosticCopySectionDefinition[] = [
  { label: "Delivery Error", value: (run) => optionalText(run.delivery_error) },
  { label: "Error", value: (run) => optionalText(run.error_message) },
  { label: "Summary", value: (run) => optionalText(run.result_summary) },
  { label: "Result", value: (run) => optionalText(run.result_text) },
  { label: "Assistant", value: assistantText },
];

export function getRunDiagnosticRows(run: ScheduledTaskRunItem): RunDiagnosticRow[] {
  return RUN_DIAGNOSTIC_ROW_DEFINITIONS.flatMap((definition) => {
    const value = definition.value(run);
    return value === null
      ? []
      : [{ breakAll: definition.breakAll, label: definition.label, value }];
  });
}

export function getRunOutputSections(run: ScheduledTaskRunItem): RunOutputSection[] {
  return RUN_OUTPUT_SECTION_DEFINITIONS.flatMap((definition) => {
    const content = definition.content(run);
    return content === null
      ? []
      : [{ content, label: definition.label, tone: definition.tone }];
  });
}

export function buildRunDiagnostic(
  task: ScheduledTaskItem,
  run: ScheduledTaskRunItem,
): string {
  const fields = DIAGNOSTIC_COPY_FIELD_DEFINITIONS.map((definition) => (
    `${definition.label}: ${definition.value(task, run)}`
  ));
  const sections = DIAGNOSTIC_COPY_SECTION_DEFINITIONS.flatMap((definition) => {
    const value = definition.value(run);
    return value === null ? [] : ["", `${definition.label}:`, value];
  });
  return [...fields, ...sections].join("\n");
}
