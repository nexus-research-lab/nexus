import type { ImageContent } from "@/types/conversation/message";
import type { PermissionUpdate } from "@/types/conversation/permission";
import { formatTokens } from "@/lib/utils";

import type {
  ToolBlockProps,
  ToolBlockStatus,
  ToolBlockViewModel,
  ToolPermissionSuggestion,
  ToolStatusTone,
} from "./tool-block-types";

const TOOL_TITLE_MAP: Record<string, string> = {
  Bash: "执行命令",
  Read: "读取内容",
  Write: "写入内容",
  Edit: "修改内容",
  MultiEdit: "批量修改",
  Grep: "查找内容",
  Glob: "浏览文件",
  LS: "查看目录",
  TodoWrite: "更新计划",
  AskUserQuestion: "等待你的确认",
  WebSearch: "网络搜索",
  WebFetch: "抓取网页",
  Skill: "调用技能",
  Task: "委派任务",
};

export const FIELD_LABEL_MAP: Record<string, string> = {
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

const INPUT_SUMMARY_KEYS = [
  "file_path",
  "path",
  "url",
  "query",
  "pattern",
  "description",
  "task",
  "prompt",
] as const;

export const TOOL_DETAIL_SCROLL_CLASS_NAME =
  "min-w-0 max-h-[18rem] overflow-auto overscroll-contain custom-scrollbar";

export const TOOL_TONE_STYLES: Record<ToolStatusTone, string> = {
  default: "text-(--icon-muted)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--warning)",
};

export const TOOL_LABEL_STYLES: Record<ToolStatusTone, string> = {
  default: "text-(--text-default)",
  error: "text-(--destructive)",
  running: "text-(--primary)",
  success: "text-(--success)",
  waiting: "text-(--warning)",
};

export function getToolTitle(toolName: string): string {
  return TOOL_TITLE_MAP[toolName] ?? toolName;
}

function formatPermissionValue(value: unknown): string {
  if (value == null || value === "") return "空";
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  if (Array.isArray(value)) {
    return value.map((item) => formatPermissionValue(item)).join("、");
  }
  if (typeof value === "object") {
    return Object.entries(value as Record<string, unknown>)
      .map(([key, nestedValue]) => `${FIELD_LABEL_MAP[key] || key}：${formatPermissionValue(nestedValue)}`)
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

export function getInputSummary(input: unknown): string | null {
  const record = asRecord(input);
  if (!record) return null;
  for (const key of INPUT_SUMMARY_KEYS) {
    const value = getStringField(record, key);
    if (value) return value;
  }
  const command = getStringField(record, "command");
  if (command) {
    return `$ ${command.slice(0, 50)}${command.length > 50 ? "..." : ""}`;
  }
  return null;
}

function getPrimaryInputDetail(input: unknown): { key: string; value: string } | null {
  const record = asRecord(input);
  if (!record) return null;
  for (const key of PRIMARY_INPUT_KEYS) {
    const value = getStringField(record, key);
    if (value) {
      return { key, value };
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

export function isImageContent(value: unknown): value is ImageContent {
  return Boolean(
    value &&
    typeof value === "object" &&
    (value as { type?: unknown }).type === "image",
  );
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
  const inputSummary = getInputSummary(toolUse.input);
  const primaryInputDetail = getPrimaryInputDetail(
    permissionRequest?.tool_input ?? toolUse.input,
  );
  const permissionFields = Object.entries(permissionRequest?.tool_input ?? {})
    .filter(([key]) => key !== primaryInputDetail?.key)
    .map(([key, value]) => ({
      label: FIELD_LABEL_MAP[key] || key,
      value: formatPermissionValue(value),
    }));
  const permissionFieldSummary = permissionFields.length > 0
    ? permissionFields.map((field) => `${field.label}：${field.value}`).join(" · ")
    : null;
  const resultSummary = toolResult ? getResultSummary(toolResult.content) : null;
  const expandedInputDetail = getPrimaryInputDetail(toolUse.input);
  const isWaiting = finalStatus === "waiting_permission";

  return {
    collapsedDetailText:
      (isWaiting && permissionFieldSummary) ||
      inputSummary ||
      resultSummary,
    durationText: formatDuration(startTime, endTime),
    expandedDetailText:
      (isWaiting && permissionFieldSummary) ||
      expandedInputDetail?.value.trim() ||
      inputSummary ||
      resultSummary,
    hasResult: Boolean(toolResult),
    isError: finalStatus === "error",
    isRunning: finalStatus === "running",
    isSuccess: finalStatus === "success",
    isWaiting,
    liveStatusText: formatLiveProgress(liveProgress),
    primaryInputDetail,
    readableSuggestions: getReadableSuggestions(permissionRequest?.suggestions),
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
