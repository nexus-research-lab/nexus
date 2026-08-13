/**
 * INPUT: Tool use/result、运行状态、权限请求与本地化上下文。
 * OUTPUT: 区分 transport error、semantic rejection、superseded 与 success 的工具卡片视图模型。
 * POS: DM/Room 共用 ToolBlock 的纯展示投影，不决定 Agent 的恢复路线。
 */
import type { PermissionUpdate } from "@/types/conversation/interaction/permission";
import { formatTokens } from "@/lib/format/token-count";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ToolResultContent } from "@/types/conversation/message/content";

import {
  getCompactToolInputSummary,
  getToolInputSummary,
  getLocalizedToolTitle as resolveLocalizedToolTitle,
} from "../../tool-activity";
import { projectToolResultMutation } from "../../tool-result-semantic-model";
import type {
  ToolBlockProps,
  ToolBlockStatus,
  ToolBlockViewModel,
  ToolPermissionRequest,
  ToolPermissionSuggestion,
  ToolPrimaryInputDetail,
  ToolStatusTone,
} from "./tool-block-types";

const FIELD_LABEL_KEY_MAP: Readonly<Record<string, TranslationKey>> = {
  answers: "message.tool_field_answers",
  command: "message.tool_field_command",
  description: "message.tool_field_description",
  directories: "message.tool_field_directories",
  file_path: "message.tool_field_file_path",
  mode: "message.tool_field_mode",
  path: "message.tool_field_path",
  pattern: "message.tool_field_pattern",
  prompt: "message.tool_field_prompt",
  query: "message.tool_field_query",
  task: "message.tool_field_task",
  url: "message.tool_field_url",
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

const DESTINATION_LABEL_KEY_MAP: Readonly<Record<string, TranslationKey>> = {
  localSettings: "message.tool_destination_local_settings",
  projectSettings: "message.tool_destination_project_settings",
  session: "message.tool_destination_session",
  userSettings: "message.tool_destination_user_settings",
};

const BEHAVIOR_LABEL_KEY_MAP: Readonly<Record<string, TranslationKey>> = {
  allow: "message.tool_behavior_allow",
  ask: "message.tool_behavior_ask",
  deny: "message.tool_behavior_deny",
};

const STATUS_META: Readonly<Record<
  ToolBlockStatus,
  { badgeClassName: string; labelKey: TranslationKey; tone: ToolStatusTone }
>> = {
  error: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)",
    labelKey: "message.tool_status_error",
    tone: "error",
  },
  pending: {
    badgeClassName: "bg-primary/10 text-primary",
    labelKey: "message.tool_status_pending",
    tone: "default",
  },
  running: {
    badgeClassName: "bg-primary/10 text-primary",
    labelKey: "message.tool_status_running",
    tone: "running",
  },
  rejected: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)",
    labelKey: "message.tool_status_rejected",
    tone: "error",
  },
  superseded: {
    badgeClassName: "bg-(--surface-muted-background) text-(--text-muted)",
    labelKey: "message.tool_status_superseded",
    tone: "default",
  },
  stopped: {
    badgeClassName: "bg-(--surface-muted-background) text-(--text-muted)",
    labelKey: "message.tool_status_stopped",
    tone: "default",
  },
  success: {
    badgeClassName: "bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
    labelKey: "message.tool_status_success",
    tone: "success",
  },
  waiting_permission: {
    badgeClassName: "border border-(--divider-subtle-color) bg-transparent text-(--text-muted)",
    labelKey: "message.tool_status_waiting_permission",
    tone: "waiting",
  },
};

interface PermissionProjection {
  fieldSummary: string | null;
  primaryInputDetail: ToolPrimaryInputDetail | null;
  readableSuggestions: ToolPermissionSuggestion[];
}

interface PermissionValueFormatter {
  format: (value: unknown, localization: ToolBlockLocalization) => string;
  matches: (value: unknown) => boolean;
}

type ToolBlockLocalization = Pick<I18nContextValue, "locale" | "t">;

const PERMISSION_VALUE_FORMATTERS: ReadonlyArray<PermissionValueFormatter> = [
  {
    matches: (value) => value == null || value === "",
    format: (_value, { t }) => t("message.tool_value_empty"),
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
    format: (value, localization) => (value as unknown[])
      .map((item) => formatPermissionValue(item, localization))
      .join(localization.locale === "zh" ? "、" : ", "),
  },
  {
    matches: isObjectValue,
    format: (value, localization) => Object.entries(
      value as Record<string, unknown>,
    )
      .map(([key, nestedValue]) => localization.t(
        "message.tool_field_pair",
        {
          label: getFieldLabel(key, localization),
          value: formatPermissionValue(nestedValue, localization),
        },
      ))
      .join(localization.locale === "zh" ? "；" : "; "),
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
  rejected: () => null,
  superseded: () => null,
  stopped: () => null,
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
  localization,
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
> & { localization: ToolBlockLocalization }): ToolBlockViewModel {
  const { t } = localization;
  const finalStatus = resolveFinalStatus(toolResult, status);
  const statusMeta = STATUS_META[finalStatus];
  const permission = buildPermissionProjection(localization, permissionRequest);
  const collapsedInputSummary = getCompactToolInputSummary(toolUse.input);
  const expandedInputSummary = getToolInputSummary(toolUse.input);
  const resultSummary = projectOptional(
    toolResult,
    (result) => getResultSummary(result, localization),
  );
  const expandedInputDetail = getPrimaryToolInputDetail(
    toolUse.input,
    localization,
  );
  const waitingDetail = WAITING_DETAIL_BY_STATUS[finalStatus](permission);
  const terminalDetail = ["rejected", "superseded"].includes(finalStatus)
    ? resultSummary
    : null;

  return {
    collapsedDetailText: firstText([
      waitingDetail,
      terminalDetail,
      collapsedInputSummary,
      resultSummary,
    ]),
    durationText: formatDuration(startTime, endTime),
    expandedDetailText: firstText([
      waitingDetail,
      terminalDetail,
      expandedInputDetail?.value.trim(),
      expandedInputSummary,
      resultSummary,
    ]),
    hasResult: Boolean(toolResult),
    liveStatusText: formatLiveProgress(liveProgress, localization),
    primaryInputDetail: permission.primaryInputDetail,
    readableSuggestions: permission.readableSuggestions,
    status: finalStatus,
    statusBadgeClassName: statusMeta.badgeClassName,
    statusText: t(statusMeta.labelKey),
    statusTone: statusMeta.tone,
    toolTitle: getLocalizedToolTitle(toolUse.name, localization),
    waitingActionHint: formatWaitingActionHint(
      interactionDisabled,
      interactionDisabledReason,
      permissionRequest?.expires_at,
      localization,
    ),
  };
}

function resolveFinalStatus(
  result: ToolResultContent | undefined,
  status: ToolBlockStatus,
): ToolBlockStatus {
  const rules = [
    { matches: Boolean(result?.is_error), value: "error" as const },
    {
      matches: projectToolResultMutation(result)?.outcome === "rejected",
      value: "rejected" as const,
    },
    {
      matches: projectToolResultMutation(result)?.outcome === "superseded",
      value: "superseded" as const,
    },
    { matches: true, value: status },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function formatPermissionValue(
  value: unknown,
  localization: ToolBlockLocalization,
): string {
  return PERMISSION_VALUE_FORMATTERS
    .find((formatter) => formatter.matches(value))!
    .format(value, localization);
}

export function getReadablePermissionSuggestions(
  suggestions: PermissionUpdate[] = [],
  localization: ToolBlockLocalization,
): ToolPermissionSuggestion[] {
  return suggestions.map((suggestion, index) => {
    const destination = resolveMappedLabel(
      suggestion.destination,
      DESTINATION_LABEL_KEY_MAP,
      "message.tool_destination_current_session",
      localization,
    );
    const behavior = resolveMappedLabel(
      suggestion.behavior,
      BEHAVIOR_LABEL_KEY_MAP,
      "message.tool_behavior_update_rule",
      localization,
    );
    return {
      index,
      label: buildSuggestionLabel(
        suggestion.behavior,
        behavior,
        destination,
        localization,
      ),
    };
  });
}

function resolveMappedLabel(
  value: string | undefined,
  labelKeys: Readonly<Record<string, TranslationKey>>,
  fallbackKey: TranslationKey,
  { t }: ToolBlockLocalization,
): string {
  if (!value) {
    return t(fallbackKey);
  }
  const labelKey = labelKeys[value];
  return labelKey ? t(labelKey) : value;
}

function buildSuggestionLabel(
  behaviorValue: string | undefined,
  behavior: string,
  destination: string,
  { t }: ToolBlockLocalization,
): string {
  return behaviorValue === "allow"
    ? t("message.tool_suggestion_write", { destination })
    : t("message.tool_suggestion_behavior", { behavior, destination });
}

export function getPrimaryToolInputDetail(
  input: unknown,
  localization: ToolBlockLocalization,
): ToolPrimaryInputDetail | null {
  const record = asRecord(input);
  if (!record) {
    return null;
  }
  for (const key of PRIMARY_INPUT_KEYS) {
    const value = getStringField(record, key);
    if (value) {
      return { key, label: getFieldLabel(key, localization), value };
    }
  }
  return null;
}

function getResultSummary(
  result: ToolResultContent,
  { t }: ToolBlockLocalization,
): string {
  const mutation = projectToolResultMutation(result);
  if (mutation?.outcome === "rejected") {
    return mutation.message || t("message.tool_rejection_without_detail");
  }
  if (mutation?.outcome === "superseded") {
    return mutation.message || t("message.tool_superseded_without_detail");
  }
  const content = result.content;
  if (typeof content === "string") {
    return truncateResultSummary(content);
  }
  return t("message.tool_json_data");
}

function truncateResultSummary(content: string): string {
  if (content.length <= 80) {
    return content;
  }
  return `${content.slice(0, 80)}...`;
}

function buildPermissionProjection(
  localization: ToolBlockLocalization,
  permissionRequest?: ToolPermissionRequest,
): PermissionProjection {
  if (!permissionRequest) {
    return {
      fieldSummary: null,
      primaryInputDetail: null,
      readableSuggestions: [],
    };
  }
  const primaryInputDetail = getPrimaryToolInputDetail(
    permissionRequest.tool_input,
    localization,
  );
  const fields = Object.entries(permissionRequest.tool_input)
    .filter(([key]) => key !== primaryInputDetail?.key)
    .map(([key, value]) => ({
      label: getFieldLabel(key, localization),
      value: formatPermissionValue(value, localization),
    }));
  return {
    fieldSummary: firstText([
      fields.map((field) => localization.t("message.tool_field_pair", field))
        .join(" · "),
    ]),
    primaryInputDetail,
    readableSuggestions: getReadablePermissionSuggestions(
      permissionRequest.suggestions,
      localization,
    ),
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
  localization: ToolBlockLocalization,
): string | null {
  if (!liveProgress) {
    return null;
  }
  return firstText([
    [
      formatCurrentToolName(liveProgress.last_tool_name, localization),
      formatLiveTokenCount(liveProgress.usage?.total_tokens),
    ].filter(Boolean).join(" · "),
  ]);
}

function formatCurrentToolName(
  toolName: string | null | undefined,
  localization: ToolBlockLocalization,
): string | null {
  if (!toolName) {
    return null;
  }
  return localization.t("message.tool_current", {
    tool: getLocalizedToolTitle(toolName, localization),
  });
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
  localization: ToolBlockLocalization,
): string {
  const rules = [
    {
      matches: interactionDisabled,
      value: firstText([
        interactionDisabledReason,
        localization.t("message.tool_unavailable"),
      ])!,
    },
    {
      matches: true,
      value: formatPermissionDeadline(expiresAt, localization),
    },
  ];
  return rules.find((rule) => rule.matches)!.value;
}

function formatPermissionDeadline(
  expiresAt: string | undefined,
  localization: ToolBlockLocalization,
): string {
  return expiresAt
    ? localization.t("message.tool_confirm_before", {
      time: new Date(expiresAt).toLocaleTimeString(
        localization.locale === "zh" ? "zh-CN" : "en-US",
      ),
    })
    : localization.t("message.tool_continue_after_confirmation");
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

function getFieldLabel(
  key: string,
  { t }: ToolBlockLocalization,
): string {
  const labelKey = FIELD_LABEL_KEY_MAP[key];
  return labelKey ? t(labelKey) : key;
}

function getLocalizedToolTitle(
  toolName: string,
  { t }: ToolBlockLocalization,
): string {
  return resolveLocalizedToolTitle(toolName, t);
}
