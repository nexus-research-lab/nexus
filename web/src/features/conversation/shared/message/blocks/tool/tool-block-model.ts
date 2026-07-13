import type { PermissionUpdate } from "@/types/conversation/interaction/permission";
import { formatTokens } from "@/lib/format/token-count";

import {
  getToolInputSummary,
  getToolTitle,
} from "../../tool-activity";
import type {
  ToolBlockProps,
  ToolBlockStatus,
  ToolBlockViewModel,
  ToolPermissionRequest,
  ToolPermissionSuggestion,
  ToolPrimaryInputDetail,
  ToolStatusTone,
} from "./tool-block-types";

const FIELD_LABEL_MAP: Readonly<Record<string, string>> = {
  answers: "回答",
  command: "命令",
  description: "说明",
  directories: "目录",
  file_path: "文件路径",
  mode: "模式",
  path: "路径",
  pattern: "匹配内容",
  prompt: "提示词",
  query: "搜索内容",
  task: "任务",
  url: "网址",
};

const PRIMARY_INPUT_KEYS = [
  "command",
  "query",
  "url",
  "path",
  "file_path",
  "pattern",
  "description",
  "prompt",
  "task",
] as const;

const DESTINATION_LABEL_MAP: Readonly<Record<string, string>> = {
  localSettings: "本地设置",
  projectSettings: "项目设置",
  session: "仅本会话",
  userSettings: "用户设置",
};

const BEHAVIOR_LABEL_MAP: Readonly<Record<string, string>> = {
  allow: "允许",
  ask: "继续询问",
  deny: "拒绝",
};

const STATUS_META: Readonly<Record<
  ToolBlockStatus,
  { badgeClassName: string; label: string; tone: ToolStatusTone }
>> = {
  error: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)",
    label: "失败",
    tone: "error",
  },
  pending: {
    badgeClassName: "bg-primary/10 text-primary",
    label: "待处理",
    tone: "default",
  },
  running: {
    badgeClassName: "bg-primary/10 text-primary",
    label: "执行中",
    tone: "running",
  },
  success: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
    label: "完成",
    tone: "success",
  },
  waiting_permission: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)",
    label: "待确认",
    tone: "waiting",
  },
};

interface PermissionProjection {
  fieldSummary: string | null;
  primaryInputDetail: ToolPrimaryInputDetail | null;
  readableSuggestions: ToolPermissionSuggestion[];
}

interface PermissionValueFormatter {
  format: (value: unknown) => string;
  matches: (value: unknown) => boolean;
}

const PERMISSION_VALUE_FORMATTERS: ReadonlyArray<PermissionValueFormatter> = [
  {
    matches: (value) => value == null || value === "",
    format: () => "空",
  },
  {
    matches: (value) => typeof value === "string",
    format: (value) => value as string,
  },
  {
    matches: (value) => ["number", "boolean"].includes(typeof value),
    format: (value) => String(value),
  },
  {
    matches: Array.isArray,
    format: (value) => (value as unknown[])
      .map((item) => formatPermissionValue(item))
      .join("、"),
  },
  {
    matches: isObjectValue,
    format: (value) => Object.entries(value as Record<string, unknown>)
      .map(([key, nestedValue]) => (
        `${getFieldLabel(key)}：${formatPermissionValue(nestedValue)}`
      ))
      .join("；"),
  },
  {
    matches: () => true,
    format: (value) => String(value),
  },
];

const WAITING_DETAIL_BY_STATUS: Readonly<Record<
  ToolBlockStatus,
  (permission: PermissionProjection) => string | null
>> = {
  error: () => null,
  pending: () => null,
  running: () => null,
  success: () => null,
  waiting_permission: (permission) => permission.fieldSummary,
};

export function buildToolBlockViewModel({
  toolUse,
  toolResult,
  liveProgress,
  status = "success",
  startTime,
  endTime,
  permissionRequest,
  interactionDisabled = false,
  interactionDisabledReason,
}: Pick<
  ToolBlockProps,
  | "endTime"
  | "interactionDisabled"
  | "interactionDisabledReason"
  | "liveProgress"
  | "permissionRequest"
  | "startTime"
  | "status"
  | "toolResult"
  | "toolUse"
>): ToolBlockViewModel {
  const finalStatus = resolveFinalStatus(Boolean(toolResult?.is_error), status);
  const statusMeta = STATUS_META[finalStatus];
  const permission = buildPermissionProjection(permissionRequest);
  const inputSummary = getToolInputSummary(toolUse.input);
  const resultSummary = projectOptional(
    toolResult,
    (result) => getResultSummary(result.content),
  );
  const expandedInputDetail = getPrimaryInputDetail(toolUse.input);
  const waitingDetail = WAITING_DETAIL_BY_STATUS[finalStatus](permission);

  return {
    collapsedDetailText: firstText([
      waitingDetail,
      inputSummary,
      resultSummary,
    ]),
    durationText: formatDuration(startTime, endTime),
    expandedDetailText: firstText([
      waitingDetail,
      expandedInputDetail?.value.trim(),
      inputSummary,
      resultSummary,
    ]),
    hasResult: Boolean(toolResult),
    liveStatusText: formatLiveProgress(liveProgress),
    primaryInputDetail: permission.primaryInputDetail,
    readableSuggestions: permission.readableSuggestions,
    status: finalStatus,
    statusBadgeClassName: statusMeta.badgeClassName,
    statusText: statusMeta.label,
    statusTone: statusMeta.tone,
    toolTitle: getToolTitle(toolUse.name),
    waitingActionHint: formatWaitingActionHint(
      interactionDisabled,
      interactionDisabledReason,
      permissionRequest?.expires_at,
    ),
  };
}

function resolveFinalStatus(
  resultIsError: boolean,
  status: ToolBlockStatus,
): ToolBlockStatus {
  const rules = [
    { matches: resultIsError, value: "error" as const },
    { matches: true, value: status },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function formatPermissionValue(value: unknown): string {
  return PERMISSION_VALUE_FORMATTERS
    .find((formatter) => formatter.matches(value))!
    .format(value);
}

function getReadableSuggestions(
  suggestions: PermissionUpdate[] = [],
): ToolPermissionSuggestion[] {
  return suggestions.map((suggestion, index) => {
    const destination = resolveMappedLabel(
      suggestion.destination,
      DESTINATION_LABEL_MAP,
      "当前会话",
    );
    const behavior = resolveMappedLabel(
      suggestion.behavior,
      BEHAVIOR_LABEL_MAP,
      "更新规则",
    );
    return {
      index,
      label: buildSuggestionLabel(behavior, destination),
    };
  });
}

function resolveMappedLabel(
  value: string | undefined,
  labels: Readonly<Record<string, string>>,
  fallback: string,
): string {
  if (!value) {
    return fallback;
  }
  return labels[value] ?? value;
}

function buildSuggestionLabel(behavior: string, destination: string): string {
  const formatters = [
    () => `${behavior}并写入${destination}`,
    () => `写入${destination}`,
  ];
  return formatters[Number(behavior === "允许")]();
}

function getPrimaryInputDetail(input: unknown): ToolPrimaryInputDetail | null {
  const record = asRecord(input);
  if (!record) {
    return null;
  }
  for (const key of PRIMARY_INPUT_KEYS) {
    const value = getStringField(record, key);
    if (value) {
      return { key, label: getFieldLabel(key), value };
    }
  }
  return null;
}

function getResultSummary(content: unknown): string {
  if (typeof content === "string") {
    return truncateResultSummary(content);
  }
  return "JSON 数据";
}

function truncateResultSummary(content: string): string {
  if (content.length <= 80) {
    return content;
  }
  return `${content.slice(0, 80)}...`;
}

function buildPermissionProjection(
  permissionRequest?: ToolPermissionRequest,
): PermissionProjection {
  if (!permissionRequest) {
    return {
      fieldSummary: null,
      primaryInputDetail: null,
      readableSuggestions: [],
    };
  }
  const primaryInputDetail = getPrimaryInputDetail(permissionRequest.tool_input);
  const fields = Object.entries(permissionRequest.tool_input)
    .filter(([key]) => key !== primaryInputDetail?.key)
    .map(([key, value]) => ({
      label: getFieldLabel(key),
      value: formatPermissionValue(value),
    }));
  return {
    fieldSummary: firstText([
      fields.map((field) => `${field.label}：${field.value}`).join(" · "),
    ]),
    primaryInputDetail,
    readableSuggestions: getReadableSuggestions(permissionRequest.suggestions),
  };
}

function formatDuration(startTime?: number, endTime?: number): string {
  if (!startTime) {
    return "";
  }
  const duration = resolveEndTime(endTime) - startTime;
  if (duration <= 0) {
    return "";
  }
  const formatters = [
    { matches: duration >= 1000, format: () => `${(duration / 1000).toFixed(1)}s` },
    { matches: true, format: () => `${duration}ms` },
  ];
  return formatters.find((formatter) => formatter.matches)!.format();
}

function resolveEndTime(endTime?: number): number {
  return endTime ?? Date.now();
}

function formatLiveProgress(
  liveProgress: ToolBlockProps["liveProgress"],
): string | null {
  if (!liveProgress) {
    return null;
  }
  return firstText([
    [
      formatCurrentToolName(liveProgress.last_tool_name),
      formatLiveTokenCount(liveProgress.usage?.total_tokens),
    ].filter(Boolean).join(" · "),
  ]);
}

function formatCurrentToolName(toolName?: string | null): string | null {
  if (!toolName) {
    return null;
  }
  return `当前 ${toolName}`;
}

function formatLiveTokenCount(totalTokens: unknown): string | null {
  if (typeof totalTokens !== "number") {
    return null;
  }
  if (totalTokens <= 0) {
    return null;
  }
  return formatTokens(totalTokens);
}

function formatWaitingActionHint(
  interactionDisabled: boolean,
  interactionDisabledReason: string | undefined,
  expiresAt: string | undefined,
): string {
  const rules = [
    {
      matches: interactionDisabled,
      value: firstText([interactionDisabledReason, "当前暂不可操作"])!,
    },
    { matches: true, value: formatPermissionDeadline(expiresAt) },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function formatPermissionDeadline(expiresAt?: string): string {
  return expiresAt
    ? `${new Date(expiresAt).toLocaleTimeString()} 前确认`
    : "确认后继续执行";
}

function projectOptional<Input, Output>(
  value: Input | undefined,
  project: (input: Input) => Output,
): Output | null {
  if (value === undefined) {
    return null;
  }
  return project(value);
}

function firstText(candidates: Array<string | null | undefined>): string | null {
  return candidates.find(Boolean) ?? null;
}

function isObjectValue(value: unknown): boolean {
  return [
    value !== null,
    typeof value === "object",
    !Array.isArray(value),
  ].every(Boolean);
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return isObjectValue(value) ? value as Record<string, unknown> : null;
}

function getStringField(
  record: Record<string, unknown>,
  key: string,
): string | null {
  const value = record[key];
  return [typeof value === "string", Boolean(value)].every(Boolean)
    ? value as string
    : null;
}

function getFieldLabel(key: string): string {
  return FIELD_LABEL_MAP[key] ?? key;
}
