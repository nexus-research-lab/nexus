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
import { useEffect, useMemo, useRef, useState } from "react";

import {
  getPrimaryToolInputDetail,
  getReadablePermissionSuggestions,
} from "@/features/conversation/shared/message/blocks/tool/tool-block-model";
import {
  getToolInputSummary,
} from "@/features/conversation/shared/message/tool-activity";
import {
  createConfigurationSecretDraft,
  getConfigurationSecretDraftValues,
  hasCompleteConfigurationSecrets,
  selectConfigurationSecrets,
  updateConfigurationSecretDraft,
} from "@/lib/conversation/configuration-secret-permission";
import {
  type I18nContextValue,
  useI18n,
} from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiSplitButton } from "@/shared/ui/button/split-button";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiInput } from "@/shared/ui/form/form-control";
import {
  UiActionMenu,
} from "@/shared/ui/menu/action-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import type { ComposerInteractionKind } from "./composer-interaction-model";
import {
  getPermissionToolTitleKey,
} from "./composer-permission-model";
import {
  ALLOW_ONCE_MENU_VALUE,
  ALLOW_TASK_MENU_VALUE,
  buildComposerPermissionScopeItems,
} from "./composer-permission-scope-items";

export interface ComposerPermissionSurfaceProps {
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
  const [secretDraft, setSecretDraft] = useState(() =>
    createConfigurationSecretDraft(permission.request_id));
  const scopeMenuAnchorRef = useRef<HTMLButtonElement>(null);
  const presentation = buildPermissionPresentation(
    permission,
    kind,
    localization,
  );
  const secretSlots = permission.configuration_secret_slots ?? [];
  const secretValues = getConfigurationSecretDraftValues(
    secretDraft,
    permission.request_id,
  );
  const hasCompleteSecrets = hasCompleteConfigurationSecrets(
    secretSlots,
    secretValues,
  );
  useEffect(() => {
    setSecretDraft(createConfigurationSecretDraft(permission.request_id));
  }, [permission.request_id]);
  const scopeItems = useMemo(
    () => buildComposerPermissionScopeItems(
      permission,
      presentation.suggestions,
      requesterName,
      t,
    ),
    [
      permission,
      presentation.suggestions,
      requesterName,
      t,
    ],
  );
  const ToolIcon = presentation.icon;
  const hasScopeChoices = presentation.suggestions.length > 0
    || (permission.source === "automation" && permission.automation?.allow_task);
  const decisionWidthClassName = hasScopeChoices
    ? "w-28"
    : "w-24";
  const respond = (
    decision: PermissionDecisionPayload["decision"],
    suggestionIndex?: number,
    automationScope?: PermissionDecisionPayload["automation_scope"],
  ) => {
    const configurationSecrets = decision === "allow"
      ? selectConfigurationSecrets(secretSlots, secretValues)
      : undefined;
    if (
      decision === "allow"
      && secretSlots.length > 0
      && !configurationSecrets
    ) {
      return false;
    }
    const selectedSuggestion = suggestionIndex === undefined
      ? undefined
      : permission.suggestions?.[suggestionIndex];
    const accepted = onResponse({
      decision,
      request_id: permission.request_id,
      ...(configurationSecrets
        ? { configuration_secrets: configurationSecrets }
        : {}),
      updated_permissions: selectedSuggestion
        ? [selectedSuggestion]
        : undefined,
      ...(permission.source === "automation"
        ? { automation_scope: automationScope ?? "once" }
        : {}),
    });
    if (accepted) {
      setSecretDraft(createConfigurationSecretDraft(permission.request_id));
    }
    return accepted;
  };

  return (
    <div
      className="space-y-4"
      data-composer-permission-surface
    >
      <div className={cn(
        "flex min-w-0 items-center gap-2",
        getUiTypographyClassName({ role: "metadata", tone: "muted" }),
      )}>
        {requesterName ? (
          <>
            <UiAgentAvatar
              avatar={requesterAvatar}
              data-composer-interaction-requester
              name={requesterName}
              size="xs"
            />
            <span className={cn(
              "truncate",
              getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "medium" }),
            )}>
              {requesterName}
            </span>
            <span aria-hidden className="text-(--text-soft)">·</span>
          </>
        ) : null}
        <ToolIcon aria-hidden className="h-4 w-4 shrink-0 text-(--icon-muted)" />
        <span className="truncate">{presentation.title}</span>
        {total > 1 ? (
          <span
            className={cn(
              "ml-auto shrink-0 tabular-nums",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}
            data-composer-interaction-queue
          >
            1 / {total}
          </span>
        ) : null}
      </div>

      <div className="space-y-3">
        <p className={cn(
          "m-0",
          getUiTypographyClassName({ role: "body", tone: "strong" }),
        )}>
          {presentation.description}
        </p>
        {presentation.detail ? (
          <pre className={cn(
            "message-cjk-font m-0 max-h-28 overflow-auto whitespace-pre-wrap break-all",
            getUiTypographyClassName({ role: "code", tone: "muted" }),
          )}>
            {presentation.detail}
          </pre>
        ) : null}
      </div>

      {secretSlots.length > 0 ? (
        <fieldset
          className="surface-radius-md space-y-3 border border-(--divider-subtle-color) p-3"
          disabled={interactionDisabled}
        >
          <legend className={cn(
            "px-1",
            getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "medium" }),
          )}>
            {t("composer.permission_configuration_secrets_title")}
          </legend>
          <p className={cn(
            "m-0",
            getUiTypographyClassName({ role: "caption", tone: "muted" }),
          )}>
            {t("composer.permission_configuration_secrets_description")}
          </p>
          <div className="space-y-3">
            {secretSlots.map((slot) => (
              <label
                className="block space-y-1.5"
                key={slot.id}
              >
                <span className={cn(
                  "block break-all",
                  getUiTypographyClassName({ role: "caption", tone: "default", weight: "medium" }),
                )}>
                  {slot.path}
                </span>
                <UiInput
                  autoComplete="new-password"
                  controlSize="lg"
                  onChange={(event) => {
                    const value = event.currentTarget.value;
                    setSecretDraft((current) =>
                      updateConfigurationSecretDraft(
                        current,
                        permission.request_id,
                        slot.id,
                        value,
                      ));
                  }}
                  placeholder={t(
                    "composer.permission_configuration_secret_placeholder",
                  )}
                  spellCheck={false}
                  type="password"
                  value={secretValues[slot.id] ?? ""}
                  variant="surface"
                />
              </label>
            ))}
          </div>
        </fieldset>
      ) : null}

      <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
        <UiButton
          className={decisionWidthClassName}
          data-composer-permission-action="deny"
          data-composer-permission-decision="deny"
          disabled={interactionDisabled}
          onClick={() => respond("deny")}
          size="sm"
          variant="surface"
        >
          {t("composer.permission_deny")}
        </UiButton>
        <UiSplitButton
          ariaLabel={t("composer.permission_allow_once")}
          className={cn("h-11 sm:h-8", decisionWidthClassName)}
          data-composer-permission-action="allow"
          mainAction={{
            children: t("composer.permission_allow_once"),
            disabled: interactionDisabled || !hasCompleteSecrets,
            onClick: () => respond("allow"),
            "data-composer-permission-decision": "allow",
          }}
          menuAction={hasScopeChoices ? {
            "aria-expanded": isScopeMenuOpen,
            "aria-haspopup": "menu",
            "aria-label": t("composer.permission_choose_scope"),
            children: <ChevronDown aria-hidden className="h-4 w-4" />,
            disabled: interactionDisabled || !hasCompleteSecrets,
            onClick: () => setIsScopeMenuOpen((current) => !current),
          } : undefined}
          menuButtonRef={scopeMenuAnchorRef}
        />
        <UiActionMenu
          align="end"
          anchorRef={scopeMenuAnchorRef}
          ariaLabel={t("composer.permission_scope_menu")}
          isOpen={isScopeMenuOpen}
          items={scopeItems}
          minWidth={228}
          onClose={() => setIsScopeMenuOpen(false)}
          onSelect={(value) => {
            if (value === ALLOW_ONCE_MENU_VALUE) {
              respond("allow");
              return;
            }
            if (value === ALLOW_TASK_MENU_VALUE) {
              respond("allow", undefined, "task");
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
