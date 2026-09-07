// INPUT: 不可变运行记录、当前任务与调用方的当前语言。
// OUTPUT: 运行输出、诊断行、区分当前配置/历史执行者的复制文本与结构化 Session 证明的文件归属。
// POS: Scheduled 历史纯投影；当前任务或当前选择不能替代历史 run 身份。

import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import { parseSessionKey } from "@/lib/conversation/session-key";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { formatScheduledDatetime } from "../scheduled-formatters";
import { getScheduledTaskErrorCopy } from "../scheduled-task-error-copy";
import { formatDuration } from "./scheduled-task-run-history-model";

type HistoryI18n = Pick<I18nContextValue, "locale" | "t">;
type Translate = I18nContextValue["t"];

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
  value: (run: ScheduledTaskRunItem, i18n: HistoryI18n) => string | null;
}

interface RunOutputSectionDefinition {
  content: (run: ScheduledTaskRunItem, t: Translate) => string | null;
  label?: string;
  tone: RunOutputSection["tone"];
}

interface DiagnosticCopyFieldDefinition {
  label: string;
  value: (task: ScheduledTaskItem, run: ScheduledTaskRunItem, i18n: HistoryI18n) => string;
}

interface DiagnosticCopySectionDefinition {
  label: string;
  value: (run: ScheduledTaskRunItem) => string | null;
}

const formatDatetime = (value: number | null, { locale, t }: HistoryI18n): string => (
  formatScheduledDatetime(value, { includeSeconds: true, locale, emptyLabel: t("capability.scheduled_history_not_recorded") })
);

const optionalText = (value: string | null | undefined): string | null => value || null;

const optionalNumber = (value: number | null | undefined): string | null => (
  typeof value === "number" ? String(value) : null
);

const positiveNumber = (value: number | null | undefined): string | null => (
  value ? String(value) : null
);

const optionalDatetime = (value: number | null, i18n: HistoryI18n): string | null => (
  value === null ? null : formatDatetime(value, i18n)
);

function assistantText(run: ScheduledTaskRunItem): string | null {
  const content = run.assistant_text?.trim();
  return content && ![run.result_text, run.result_summary].some((value) => (
    content === value?.trim()
  )) ? run.assistant_text ?? null : null;
}

function primaryResultText(run: ScheduledTaskRunItem): string | null {
  return [run.result_text, run.assistant_text, run.result_summary].find((value) => (
    Boolean(value?.trim())
  )) ?? null;
}

const RUN_DIAGNOSTIC_ROW_DEFINITIONS: readonly RunDiagnosticRowDefinition[] = [
  { breakAll: true, label: "Run", value: (run) => run.run_id },
  { label: "Trigger", value: (run) => optionalText(run.trigger_kind) },
  { label: "Messages", value: (run) => optionalNumber(run.message_count) },
  { breakAll: true, label: "Session", value: (run) => optionalText(run.session_key) },
  { breakAll: true, label: "Round", value: (run) => optionalText(run.round_id) },
  { breakAll: true, label: "Runtime", value: (run) => optionalText(run.session_id) },
  { breakAll: true, label: "Delivery", value: (run) => optionalText(run.delivery_to) },
  { label: "Delivered", value: (run, i18n) => optionalDatetime(run.delivered_at, i18n) },
  { label: "Delivery attempts", value: (run) => positiveNumber(run.delivery_attempts) },
  { label: "Next delivery retry", value: (run, i18n) => optionalDatetime(run.delivery_next_attempt_at, i18n) },
  { label: "Delivery dead letter", value: (run, i18n) => optionalDatetime(run.delivery_dead_letter_at, i18n) },
  { label: "Started", value: (run, i18n) => formatDatetime(run.started_at, i18n) },
  { label: "Finished", value: (run, i18n) => formatDatetime(run.finished_at, i18n) },
  { label: "Attempts", value: (run) => String(run.attempts) },
];

const RUN_OUTPUT_SECTION_DEFINITIONS: readonly RunOutputSectionDefinition[] = [
  {
    content: (run) => getScheduledTaskErrorCopy(run.error_message)?.detail ?? null,
    tone: "danger",
  },
  {
    content: (run, t) => run.delivery_error ? t("capability.scheduled_history_delivery_error", { error: run.delivery_error }) : null,
    tone: "danger",
  },
  { content: primaryResultText, tone: "default" },
];

const DIAGNOSTIC_COPY_FIELD_DEFINITIONS: readonly DiagnosticCopyFieldDefinition[] = [
  { label: "Task", value: (task) => task.name },
  { label: "Job ID", value: (task) => task.job_id },
  { label: "Current Task Agent ID", value: (task) => task.agent_id },
  { label: "Current Execution Kind", value: (task) => task.execution_kind ?? "agent" },
  { label: "Run ID", value: (_task, run) => run.run_id },
  { label: "Run Agent ID", value: (_task, run) => getRunWorkspaceAgentID(run) ?? "" },
  { label: "Status", value: (_task, run) => run.status },
  { label: "Delivery Status", value: (_task, run) => run.delivery_status || "" },
  { label: "Delivery Attempts", value: (_task, run) => String(run.delivery_attempts ?? 0) },
  { label: "Delivered At", value: (_task, run, i18n) => formatDatetime(run.delivered_at, i18n) },
  { label: "Delivery Next Attempt", value: (_task, run, i18n) => formatDatetime(run.delivery_next_attempt_at, i18n) },
  { label: "Delivery Dead Letter At", value: (_task, run, i18n) => formatDatetime(run.delivery_dead_letter_at, i18n) },
  { label: "Trigger", value: (_task, run) => run.trigger_kind || "" },
  { label: "Scheduled", value: (_task, run, i18n) => formatDatetime(run.scheduled_for, i18n) },
  { label: "Started", value: (_task, run, i18n) => formatDatetime(run.started_at, i18n) },
  { label: "Finished", value: (_task, run, i18n) => formatDatetime(run.finished_at, i18n) },
  {
    label: "Duration",
    value: (_task, run, { t }) => formatDuration(run.started_at, run.finished_at, t),
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

export function getRunDiagnosticRows(run: ScheduledTaskRunItem, i18n: HistoryI18n): RunDiagnosticRow[] {
  return RUN_DIAGNOSTIC_ROW_DEFINITIONS.flatMap((definition) => {
    const value = definition.value(run, i18n);
    return value === null
      ? []
      : [{ breakAll: definition.breakAll, label: definition.label, value }];
  });
}

export function getRunOutputSections(run: ScheduledTaskRunItem, t: Translate): RunOutputSection[] {
  return RUN_OUTPUT_SECTION_DEFINITIONS.flatMap((definition) => {
    const content = definition.content(run, t);
    return content === null
      ? []
      : [{ content, label: definition.label, tone: definition.tone }];
  });
}

export function getRunWorkspaceAgentID(run: ScheduledTaskRunItem): string | null {
  const session = parseSessionKey(run.session_key);
  return session.is_structured && session.kind === "agent" ? session.agent_id : null;
}

export function buildRunDiagnostic(
  task: ScheduledTaskItem,
  run: ScheduledTaskRunItem,
  i18n: HistoryI18n,
): string {
  const fields = DIAGNOSTIC_COPY_FIELD_DEFINITIONS.map((definition) => (
    `${definition.label}: ${definition.value(task, run, i18n)}`
  ));
  const sections = DIAGNOSTIC_COPY_SECTION_DEFINITIONS.flatMap((definition) => {
    const value = definition.value(run);
    return value === null ? [] : ["", `${definition.label}:`, value];
  });
  return [...fields, ...sections].join("\n");
}
