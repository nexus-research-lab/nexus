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

const FIELD_LABEL_MAP: Record<string, string> = {
  query: "搜索内容",
  url: "网址",
  command: "命令",
  path: "路径",
  file_path: "文件路径",
  pattern: "匹配内容",
  prompt: "提示词",
  description: "说明",
  task: "任务",
  mode: "模式",
  directories: "目录",
  answers: "回答",
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

function formatPermissionValue(value: unknown): string {
  if (value == null || value === "") return "空";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => formatPermissionValue(item)).join("、");
  }
  if (typeof value === "object") {
    return Object.entries(value as Record<string, unknown>)
      .map(([key, nestedValue]) => `${getFieldLabel(key)}：${formatPermissionValue(nestedValue)}`)
      .join("；");
  }
  return String(value);
}

function getReadableSuggestions(
  suggestions: PermissionUpdate[] = [],
): ToolPermissionSuggestion[] {
  const destinationMap: Record<string, string> = {
    session: "仅本会话",
    projectSettings: "项目设置",
    userSettings: "用户设置",
    localSettings: "本地设置",
  };
  const behaviorMap: Record<string, string> = {
    allow: "允许",
    deny: "拒绝",
    ask: "继续询问",
  };

  return suggestions.map((suggestion, index) => {
    const destination = suggestion.destination
      ? destinationMap[suggestion.destination] || suggestion.destination
      : "当前会话";
    const behavior = suggestion.behavior
      ? behaviorMap[suggestion.behavior] || suggestion.behavior
      : "更新规则";

    return {
      index,
      label: behavior === "允许"
        ? `写入${destination}`
        : `${behavior}并写入${destination}`,
    };
  });
}

function getPrimaryInputDetail(input: unknown): ToolPrimaryInputDetail | null {
  const record = asRecord(input);
  if (!record) return null;
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
    return content.slice(0, 80) + (content.length > 80 ? "..." : "");
  }
  return "JSON 数据";
}

const STATUS_META: Record<
  ToolBlockStatus,
  { badgeClassName: string; label: string; tone: ToolStatusTone }
> = {
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
  error: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)",
    label: "失败",
    tone: "error",
  },
  waiting_permission: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-(--warning)",
    label: "待确认",
    tone: "waiting",
  },
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
  const finalStatus = toolResult?.is_error ? "error" : status;
  const statusMeta = STATUS_META[finalStatus];
  const inputSummary = getToolInputSummary(toolUse.input);
  const permission = buildPermissionProjection(permissionRequest);
  const resultSummary = toolResult ? getResultSummary(toolResult.content) : null;
  const expandedInputDetail = getPrimaryInputDetail(toolUse.input);
  const isWaiting = finalStatus === "waiting_permission";
  const waitingDetail = isWaiting ? permission.fieldSummary : null;

  return {
    collapsedDetailText:
      waitingDetail ||
      inputSummary ||
      resultSummary,
    durationText: formatDuration(startTime, endTime),
    expandedDetailText:
      waitingDetail ||
      expandedInputDetail?.value.trim() ||
      inputSummary ||
      resultSummary,
    hasResult: Boolean(toolResult),
    isRunning: finalStatus === "running",
    isWaiting,
    liveStatusText: formatLiveProgress(liveProgress),
    primaryInputDetail: permission.primaryInputDetail,
    readableSuggestions: permission.readableSuggestions,
    status: finalStatus,
    statusBadgeClassName: statusMeta.badgeClassName,
    statusText: statusMeta.label,
    statusTone: statusMeta.tone,
    toolTitle: getToolTitle(toolUse.name),
    waitingActionHint: interactionDisabled
      ? interactionDisabledReason || "当前暂不可操作"
      : formatPermissionDeadline(permissionRequest?.expires_at),
  };
}

function buildPermissionProjection(
  permissionRequest?: ToolPermissionRequest,
): {
  fieldSummary: string | null;
  primaryInputDetail: ToolPrimaryInputDetail | null;
  readableSuggestions: ToolPermissionSuggestion[];
} {
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
    fieldSummary: fields.length > 0
      ? fields.map((field) => `${field.label}：${field.value}`).join(" · ")
      : null,
    primaryInputDetail,
    readableSuggestions: getReadableSuggestions(permissionRequest.suggestions),
  };
}

function formatDuration(startTime?: number, endTime?: number): string {
  if (!startTime) return "";
  const duration = (endTime ?? Date.now()) - startTime;
  if (duration <= 0) return "";
  return duration >= 1000 ? `${(duration / 1000).toFixed(1)}s` : `${duration}ms`;
}

function formatLiveProgress(
  liveProgress: ToolBlockProps["liveProgress"],
): string | null {
  if (!liveProgress) return null;
  const totalTokens = liveProgress.usage?.total_tokens;
  const parts = [
    liveProgress.last_tool_name ? `当前 ${liveProgress.last_tool_name}` : null,
    typeof totalTokens === "number" && totalTokens > 0
      ? formatTokens(totalTokens)
      : null,
  ].filter((part): part is string => Boolean(part));
  return parts.length > 0 ? parts.join(" · ") : null;
}

function formatPermissionDeadline(expiresAt?: string): string {
  return expiresAt
    ? `${new Date(expiresAt).toLocaleTimeString()} 前确认`
    : "确认后继续执行";
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function getStringField(
  record: Record<string, unknown>,
  key: string,
): string | null {
  const value = record[key];
  return typeof value === "string" && value ? value : null;
}

function getFieldLabel(key: string): string {
  return FIELD_LABEL_MAP[key] ?? key;
}
