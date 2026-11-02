"use client";

/**
 * INPUT: 当前 Session 设置、Agent 继承值与 Provider 模型目录。
 * OUTPUT: 模型/权限/思考强度菜单共用选项、按 Provider 分组的模型行、编码与精确选择分派。
 * POS: DM 直接菜单与 Room Agent 设置浮层之间的无状态共享层。
 */

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import {
  Check,
  FilePenLine,
  Hand,
  ListChecks,
  RotateCcw,
  ShieldAlert,
  ShieldOff,
} from "lucide-react";

import { getAgentPermissionChoices, resolveRuntimePermissionMode } from "@/lib/agent-options";
import { UiBadge } from "@/shared/ui/display/badge";
import type { SessionEffortLevel } from "@/types/capability/provider";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { useI18n } from "@/shared/i18n/i18n-context";
import type { UiActionMenuItem } from "@/shared/ui/menu/action-menu";

import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";
import type { SessionModelMenuShape } from "./composer-session-control-layout";

export const RESET_SESSION_SETTING_VALUE = "__reset__";
export const SESSION_EFFORT_AUTO_VALUE = "__auto__";

const SESSION_EFFORT_LABEL_KEYS = {
  low: "composer.session_effort_low",
  medium: "composer.session_effort_medium",
  high: "composer.session_effort_high",
  xhigh: "composer.session_effort_xhigh",
  max: "composer.session_effort_max",
} as const satisfies Record<SessionEffortLevel, string>;

const SESSION_PERMISSION_DESCRIPTION_KEYS = {
  auto: "agent_options.advanced.permission.auto.description",
  default: "composer.session_permission_default_hint",
  plan: "composer.session_permission_plan_hint",
  acceptEdits: "composer.session_permission_accept_edits_hint",
  bypassPermissions: "composer.session_permission_bypass_hint",
  dontAsk: "composer.session_permission_dont_ask_hint",
} as const;

const SESSION_PERMISSION_ICONS = {
  auto: (
    <svg aria-hidden="true" className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 3 3 7v5c0 5 4 8 9 10 5-2 9-5 9-10V7Z" />
      <path d="m8 9 3 3-3 3m5 0h3" />
    </svg>
  ),
  default: <Hand className="h-4 w-4" />,
  plan: <ListChecks className="h-4 w-4" />,
  acceptEdits: <FilePenLine className="h-4 w-4" />,
  bypassPermissions: <ShieldAlert className="h-4 w-4" />,
  dontAsk: <ShieldOff className="h-4 w-4" />,
} as const;

export function buildSessionPermissionItems(
  controller: ComposerSessionSettingsController,
  t: ReturnType<typeof useI18n>["t"],
): UiActionMenuItem[] {
  const currentMode = resolveRuntimePermissionMode(controller.settings.permission_mode
    || controller.inheritedPermissionMode, controller.scope?.runtimeKind ?? "");
  return getAgentPermissionChoices(controller.scope?.runtimeKind ?? "").map((mode) => ({
    active: currentMode === mode.value,
    description: t(SESSION_PERMISSION_DESCRIPTION_KEYS[mode.value]),
    icon: SESSION_PERMISSION_ICONS[mode.value],
    label: t(mode.labelKey),
    tone: mode.value === "bypassPermissions"
      ? "danger" as const
      : "default" as const,
    trailing: currentMode === mode.value
      ? <Check className="h-3.5 w-3.5" />
      : undefined,
    value: mode.value,
  }));
}

export interface SessionModelGroup {
  key: string;
  /** 只有多个 Provider 时才显示分组名；单一来源不重复标注。 */
  label: string | null;
  items: UiActionMenuItem[];
}

// 按 Provider 分组投影模型；继承来源的模型标记“默认”，当前生效项打勾。
export function buildSessionModelGroups(
  controller: ComposerSessionSettingsController,
  t: ReturnType<typeof useI18n>["t"],
): SessionModelGroup[] {
  const currentProvider = controller.settings.provider
    || controller.inheritedProvider;
  const currentModel = controller.settings.model || controller.inheritedModel;
  const providers = (controller.providerOptions?.items ?? [])
    .filter((provider) => provider.models.length > 0);
  return providers.map((provider) => ({
    key: provider.provider,
    label: providers.length > 1 ? provider.display_name || provider.provider : null,
    items: provider.models.map((model) => {
      const active = currentProvider === provider.provider
        && currentModel === model.model_id;
      const inherited = controller.inheritedProvider === provider.provider
        && controller.inheritedModel === model.model_id;
      const name = model.display_name || model.model_id;
      return {
        active,
        label: (
          <span className="flex min-w-0 items-center gap-2">
            <UiTooltip label={name}>
              <span className="min-w-0 truncate">{name}</span>
            </UiTooltip>
            {inherited ? (
              <UiBadge size="xs">{t("composer.session_model_default")}</UiBadge>
            ) : null}
          </span>
        ),
        trailing: active ? <Check className="h-3.5 w-3.5" /> : undefined,
        value: encodeSessionModelValue(provider.provider, model.model_id),
      };
    }),
  }));
}

export function sessionEffortLabel(
  effort: SessionEffortLevel | "",
  t: ReturnType<typeof useI18n>["t"],
): string {
  return effort ? t(SESSION_EFFORT_LABEL_KEYS[effort]) : t("composer.session_effort_auto");
}

export function buildSessionEffortItems(
  controller: ComposerSessionSettingsController,
  t: ReturnType<typeof useI18n>["t"],
): UiActionMenuItem[] {
  const levels: Array<SessionEffortLevel | ""> = ["", ...(controller.effortLevels ?? [])];
  return levels.map((level) => ({
    checked: controller.effort === level,
    label: sessionEffortLabel(level, t),
    trailing: level === "" ? (
      <span className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "regular" })}>
        {t("composer.session_effort_auto_hint")}
      </span>
    ) : level === "max" ? (
      <UiBadge size="xs" tone="warning">{t("composer.session_effort_max_hint")}</UiBadge>
    ) : undefined,
    value: level || SESSION_EFFORT_AUTO_VALUE,
  }));
}

export function applySessionEffortSelection(
  controller: Pick<ComposerSessionSettingsController, "effortLevels" | "updateEffort">,
  value: string,
): void {
  if (value === SESSION_EFFORT_AUTO_VALUE) {
    void controller.updateEffort("");
    return;
  }
  const level = (controller.effortLevels ?? []).find((candidate) => candidate === value);
  if (level) void controller.updateEffort(level);
}

export function buildResetSessionSettingItem(
  disabled: boolean,
  t: ReturnType<typeof useI18n>["t"],
): UiActionMenuItem {
  return {
    disabled,
    icon: <RotateCcw className="h-3.5 w-3.5" />,
    label: t("composer.session_reset_defaults"),
    value: RESET_SESSION_SETTING_VALUE,
  };
}

// 继承值恢复与显式 override 只在这里分派；持久化/失败对账仍由控制器负责。
export function applySessionModelSelection(
  controller: Pick<ComposerSessionSettingsController,
    "resetModel" | "updateModel">,
  value: string,
): void {
  if (value === RESET_SESSION_SETTING_VALUE) {
    void controller.resetModel();
    return;
  }
  const [provider, model] = decodeSessionModelValue(value);
  if (!provider || !model) return;
  void controller.updateModel(provider, model);
}

function decodeSessionModelValue(value: string): [string, string] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    return ["", ""];
  }
  if (
    !Array.isArray(parsed)
    || parsed.length !== 2
    || parsed.some((item) => typeof item !== "string")
  ) {
    return ["", ""];
  }
  return [parsed[0], parsed[1]];
}

function encodeSessionModelValue(provider: string, model: string): string {
  return JSON.stringify([provider, model]);
}

/** 视图投影与估算共用同一份条目结构，避免高度和 DOM 漂移。 */
export function getSessionModelMenuShape(
  controller: ComposerSessionSettingsController,
): SessionModelMenuShape {
  const groupSizes = (controller.providerOptions?.items ?? [])
    .map((provider) => provider.models.length)
    .filter((size) => size > 0);
  const effortCount = (controller.effortLevels ?? []).length;
  return {
    effortCount: effortCount > 0 ? effortCount + 1 : 0,
    // 强度入口常驻（不支持时置灰），与重置共两行。
    footerCount: 2,
    groupSizes,
  };
}
