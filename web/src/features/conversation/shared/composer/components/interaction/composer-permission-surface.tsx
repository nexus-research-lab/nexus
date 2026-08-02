"use client";

/**
 * INPUT: 当前 Composer-owned 权限/计划请求、Agent 身份与响应动作。
 * OUTPUT: 工具类型、人话摘要、必要参数和单一决策行组成的精简确认面。
 * POS: Composer 人工介入中非结构化问答请求的唯一可操作视图。
 */
import {
  ChevronDown,
  FileText,
  Globe2,
  ListChecks,
  SquareTerminal,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { useMemo, useRef, useState } from "react";

import {
  getPrimaryToolInputDetail,
  getReadablePermissionSuggestions,
} from "@/features/conversation/shared/message/blocks/tool/tool-block-model";
import {
  getToolInputSummary,
} from "@/features/conversation/shared/message/tool-activity";
import {
  type I18nContextValue,
  useI18n,
} from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { ComposerInteractionKind } from "./composer-interaction-model";
import {
  getPermissionScopeActionLabelKey,
  getPermissionScopeHintKey,
  getPermissionToolTitleKey,
  getPermissionRuleContent,
} from "./composer-permission-model";

interface ComposerPermissionSurfaceProps {
  interactionDisabled: boolean;
  kind: Exclude<ComposerInteractionKind, "question">;
  onResponse: (payload: PermissionDecisionPayload) => boolean;
  permission: PendingPermission;
  requesterAvatar?: string | null;
  requesterName?: string;
  total: number;
}

const TOOL_ICON_BY_NAME: Readonly<Record<string, LucideIcon>> = {
  Bash: SquareTerminal,
  Edit: FileText,
  MultiEdit: FileText,
  Read: FileText,
  WebFetch: Globe2,
  WebSearch: Globe2,
  Write: FileText,
};

const ALLOW_ONCE_MENU_VALUE = "allow-once";

export function ComposerPermissionSurface({
  interactionDisabled,
  kind,
  onResponse,
  permission,
  requesterAvatar,
  requesterName,
  total,
}: ComposerPermissionSurfaceProps) {
  const localization = useI18n();
  const { t } = localization;
  const [isScopeMenuOpen, setIsScopeMenuOpen] = useState(false);
  const scopeMenuAnchorRef = useRef<HTMLButtonElement>(null);
  const presentation = buildPermissionPresentation(
    permission,
    kind,
    localization,
  );
  const scopeItems = useMemo(
    () => [
      {
        label: (
          <span className="text-[15px]">
            {t("composer.permission_allow_once_menu")}
          </span>
        ),
        description: t("composer.permission_allow_once_description"),
        value: ALLOW_ONCE_MENU_VALUE,
      },
      ...presentation.suggestions.map((suggestion) => {
        const update = permission.suggestions?.[suggestion.index];
        const scopeHintKey = getPermissionScopeHintKey(update);
        const scopeHint = update?.destination === "localSettings" && requesterName
          ? t("composer.permission_scope_agent_local_settings", {
            name: requesterName,
          })
          : scopeHintKey
            ? t(scopeHintKey)
            : null;
        const actionLabelKey = getPermissionScopeActionLabelKey(
          permission.tool_name,
          update,
        );
        const ruleContent = getPermissionRuleContent(update);
        const scopeDescription = scopeHint && ruleContent
          ? t("composer.permission_rule_scope_description", {
            rule: ruleContent,
            scope: scopeHint,
          })
          : scopeHint;
        return {
          label: (
            <span className="text-[15px]">
              {actionLabelKey ? t(actionLabelKey) : suggestion.label}
            </span>
          ),
          description: scopeDescription,
          value: String(suggestion.index),
        } satisfies UiActionMenuItem;
      }),
    ],
    [
      permission.suggestions,
      permission.tool_name,
      presentation.suggestions,
      requesterName,
      t,
    ],
  );
  const ToolIcon = presentation.icon;
  const respond = (
    decision: PermissionDecisionPayload["decision"],
    suggestionIndex?: number,
  ) => {
    const selectedSuggestion = suggestionIndex === undefined
      ? undefined
      : permission.suggestions?.[suggestionIndex];
    return onResponse({
      decision,
      request_id: permission.request_id,
      updated_permissions: selectedSuggestion
        ? [selectedSuggestion]
        : undefined,
    });
  };

  return (
    <div
      className="space-y-4"
      data-composer-permission-surface
    >
      <div className="flex min-w-0 items-center gap-2 text-sm text-(--text-muted)">
        {requesterName ? (
          <>
            <UiAgentAvatar
              avatar={requesterAvatar}
              data-composer-interaction-requester
              name={requesterName}
              size="xs"
            />
            <span className="truncate font-medium text-(--text-strong)">
              {requesterName}
            </span>
            <span aria-hidden className="text-(--text-soft)">·</span>
          </>
        ) : null}
        <ToolIcon aria-hidden className="h-4 w-4 shrink-0 text-(--icon-muted)" />
        <span className="truncate">{presentation.title}</span>
        {total > 1 ? (
          <span
            className="ml-auto shrink-0 text-xs tabular-nums text-(--text-soft)"
            data-composer-interaction-queue
          >
            1 / {total}
          </span>
        ) : null}
      </div>

      <div className="space-y-3">
        <p className="m-0 text-[15px] leading-6 text-(--text-strong)">
          {presentation.description}
        </p>
        {presentation.detail ? (
          <pre className="message-cjk-font m-0 max-h-28 overflow-auto whitespace-pre-wrap break-all font-mono text-sm leading-6 text-(--text-muted)">
            {presentation.detail}
          </pre>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
        <button
          className="inline-flex h-9 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-transparent px-4 text-sm font-medium text-(--text-default) transition-colors hover:bg-(--interaction-hover-background) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
          disabled={interactionDisabled}
          onClick={() => respond("deny")}
          type="button"
        >
          {t("composer.permission_deny")}
        </button>
        <div className="flex h-9 items-stretch">
          <button
            className={cn(
              "inline-flex items-center justify-center bg-(--text-strong) px-4 text-sm font-medium text-(--background) transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
              presentation.suggestions.length > 0
                ? "rounded-l-full"
                : "rounded-full",
            )}
            disabled={interactionDisabled}
            onClick={() => respond("allow")}
            type="button"
          >
            {t("composer.permission_allow_once")}
          </button>
          {presentation.suggestions.length > 0 ? (
            <button
              ref={scopeMenuAnchorRef}
              aria-expanded={isScopeMenuOpen}
              aria-haspopup="menu"
              aria-label={t("composer.permission_choose_scope")}
              className="inline-flex w-9 items-center justify-center rounded-r-full border-l border-[color:color-mix(in_srgb,var(--background)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--text-strong)_82%,var(--background))] text-(--background) transition-[background-color,opacity] hover:bg-(--text-strong) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
              disabled={interactionDisabled}
              onClick={() => setIsScopeMenuOpen((current) => !current)}
              type="button"
            >
              <ChevronDown aria-hidden className="h-4 w-4" />
            </button>
          ) : null}
        </div>
        <UiActionMenu
          align="end"
          anchorRef={scopeMenuAnchorRef}
          ariaLabel={t("composer.permission_scope_menu")}
          className="!rounded-[1.125rem] border-[color:color-mix(in_srgb,var(--divider-subtle-color)_86%,transparent)] p-2 [&_[role=menuitem]]:rounded-xl [&_[role=menuitem]]:px-3"
          isOpen={isScopeMenuOpen}
          items={scopeItems}
          minWidth={228}
          onClose={() => setIsScopeMenuOpen(false)}
          onSelect={(value) => {
            if (value === ALLOW_ONCE_MENU_VALUE) {
              respond("allow");
              return;
            }
            const suggestionIndex = Number(value);
            if (Number.isInteger(suggestionIndex)) {
              respond("allow", suggestionIndex);
            }
          }}
          placement="top"
        />
      </div>
    </div>
  );
}

function buildPermissionPresentation(
  permission: PendingPermission,
  kind: Exclude<ComposerInteractionKind, "question">,
  localization: I18nContextValue,
) {
  const { t } = localization;
  const primaryDetail = getPrimaryToolInputDetail(
    permission.tool_input,
    localization,
  );
  const planDetail = readStringField(permission.tool_input, "plan");
  const detail = firstDistinctText(
    [planDetail, primaryDetail?.value, getToolInputSummary(permission.tool_input)],
    permission.summary,
  );
  const toolTitleKey = getPermissionToolTitleKey(permission.tool_name);
  const title = kind === "plan"
    ? t("composer.permission_plan_title")
    : toolTitleKey
      ? t(toolTitleKey)
      : permission.tool_name;
  return {
    description: permission.summary?.trim()
      || t("composer.permission_default_description", { title }),
    detail,
    icon: kind === "plan"
      ? ListChecks
      : TOOL_ICON_BY_NAME[permission.tool_name] ?? Wrench,
    suggestions: getReadablePermissionSuggestions(
      permission.suggestions,
      localization,
    ),
    title,
  };
}

function firstDistinctText(
  candidates: Array<string | null | undefined>,
  comparison?: string,
): string | null {
  const normalizedComparison = comparison?.trim();
  return candidates.find((candidate) => {
    const normalized = candidate?.trim();
    return normalized && normalized !== normalizedComparison;
  })?.trim() ?? null;
}

function readStringField(
  input: Record<string, unknown>,
  key: string,
): string | null {
  const value = input[key];
  return typeof value === "string" && value.trim()
    ? value.trim()
    : null;
}
