"use client";

/**
 * INPUT: 当前 Session 设置、Agent 继承值与 Provider 模型目录。
 * OUTPUT: 模型和权限菜单共用的选项投影与编码工具。
 * POS: DM 直接菜单与 Room Agent 设置浮层之间的无状态共享层。
 */

import {
  Check,
  FilePenLine,
  Hand,
  ListChecks,
  RotateCcw,
  ShieldAlert,
  ShieldOff,
} from "lucide-react";

import { AGENT_PERMISSION_MODES } from "@/lib/agent-options";
import type { useI18n } from "@/shared/i18n/i18n-context";
import type { UiActionMenuItem } from "@/shared/ui/menu/action-menu";

import type {
  ComposerSessionSettingsController,
} from "../../controller/use-composer-session-settings";

export const RESET_SESSION_SETTING_VALUE = "__reset__";

const SESSION_PERMISSION_DESCRIPTION_KEYS = {
  default: "composer.session_permission_default_hint",
  plan: "composer.session_permission_plan_hint",
  acceptEdits: "composer.session_permission_accept_edits_hint",
  bypassPermissions: "composer.session_permission_bypass_hint",
  dontAsk: "composer.session_permission_dont_ask_hint",
} as const;

const SESSION_PERMISSION_ICONS = {
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
  const currentMode = controller.settings.permission_mode
    || controller.inheritedPermissionMode;
  return AGENT_PERMISSION_MODES.map((mode) => ({
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

export function buildSessionModelItems(
  controller: ComposerSessionSettingsController,
): UiActionMenuItem[] {
  const currentProvider = controller.settings.provider
    || controller.inheritedProvider;
  const currentModel = controller.settings.model || controller.inheritedModel;
  const items: UiActionMenuItem[] = [];
  for (const provider of controller.providerOptions?.items ?? []) {
    for (const model of provider.models) {
      const active = currentProvider === provider.provider
        && currentModel === model.model_id;
      items.push({
        active,
        label: (
          <span className="flex min-w-0 items-center gap-2">
            <span className="truncate">
              {model.display_name || model.model_id}
            </span>
            <span className="shrink-0 text-2xs font-normal text-(--text-soft)">
              {provider.display_name || provider.provider}
            </span>
          </span>
        ),
        trailing: active ? <Check className="h-3.5 w-3.5" /> : undefined,
        value: encodeSessionModelValue(
          provider.provider,
          model.model_id,
        ),
      });
    }
  }
  return items;
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

export function decodeSessionModelValue(value: string): [string, string] {
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
