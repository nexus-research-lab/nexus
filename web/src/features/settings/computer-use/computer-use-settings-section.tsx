/**
 * INPUT: Host Computer Use installation status and permission actions.
 * OUTPUT: Dedicated Computer Use settings page with one clear enable switch.
 * POS: Settings domain for local computer control; it is separate from Runtime and Browser.
 */
"use client";

import { MousePointer2 } from "lucide-react";

import { SETTINGS_CONTENT_BODY_CLASS_NAME } from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import type { CUAInstallationPermissionKey } from "@/types/settings/preferences";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SettingsToggleRow,
} from "../shared/settings-panel-ui";
import { useComputerUseSettingsController } from "./use-computer-use-settings-controller";

const PERMISSION_ROWS: readonly [CUAInstallationPermissionKey, TranslationKey][] = [
  ["screen_capture", "settings.computer_use.permission_screen_capture"],
  ["accessibility", "settings.computer_use.permission_accessibility"],
  ["input_control", "settings.computer_use.permission_input_control"],
];

function permissionTone(value: string | undefined) {
  if (value === "granted") return "success" as const;
  if (value === "denied") return "danger" as const;
  if (value === "not_applicable") return "idle" as const;
  return "warning" as const;
}

function permissionNeedsAction(value: string | undefined) {
  return value !== "granted" && value !== "not_applicable";
}

function permissionLabel(value: string | undefined, t: (key: TranslationKey) => string) {
  if (value === "granted") return t("settings.computer_use.permission_granted");
  if (value === "denied") return t("settings.computer_use.permission_denied");
  if (value === "not_applicable") return t("settings.computer_use.permission_not_applicable");
  if (value === "not_determined") return t("settings.computer_use.permission_not_determined");
  return t("settings.computer_use.permission_unknown");
}

export function ComputerUseSettingsSection() {
  const { t } = useI18n();
  const settings = useComputerUseSettingsController();
  const installed = settings.status?.installed === true;
  const enabled = settings.status?.enabled === true;
  const permissions = settings.status?.permissions;
  const permissionsReady = enabled && permissions != null && PERMISSION_ROWS.every(([name]) => {
    return !permissionNeedsAction(permissions[name]);
  });

  return (
    <div className={`${WORKSPACE_CONTENT_PAGE_CLASS_NAME} flex flex-col`}>
      <WorkspaceContentHeader
        className="max-sm:hidden"
        title={t("settings.computer_use.section_title")}
      />
      <section className={`${SETTINGS_CONTENT_BODY_CLASS_NAME} space-y-2.5`}>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <SettingsToggleRow
            checked={enabled}
            description={t(installed
              ? "settings.computer_use.description"
              : "settings.computer_use.install_description")}
            disabled={settings.busy || settings.status === null}
            icon={<MousePointer2 className="h-3.5 w-3.5" />}
            onChange={settings.onChange}
            title={t("settings.computer_use.title")}
          />
          <div className="flex flex-wrap items-center justify-between gap-3 px-4 pb-3">
            <div className="min-w-0">
              {settings.status ? (
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} role="status">
                  {settings.status.version
                    ? t("settings.computer_use.version", { version: settings.status.version })
                    : installed
                      ? t("settings.computer_use.installed")
                      : t("settings.computer_use.not_installed")}
                </p>
              ) : null}
              {enabled ? (
                <div className="mt-3 space-y-2" aria-label={t("settings.computer_use.permissions_title")}>
                  <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} role="status">
                    {permissionsReady
                      ? t("settings.computer_use.permissions_ready")
                      : t("settings.computer_use.permissions_incomplete")}
                  </p>
                  <div className="grid gap-2 sm:grid-cols-3">
                    {PERMISSION_ROWS.map(([name, labelKey]) => {
                      const value = permissions?.[name];
                      return (
                        <div className="flex min-w-0 items-center gap-2" key={name}>
                          <span className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>{t(labelKey)}</span>
                          <UiBadge size="xs" tone={permissionTone(value)} showDot>
                            {permissionLabel(value, t)}
                          </UiBadge>
                          {permissionNeedsAction(value) ? (
                            <UiButton
                              className="shrink-0"
                              disabled={settings.busy}
                              onClick={() => void settings.onOpenPermissions(name)}
                              size="xs"
                              variant="text"
                            >
                              {t("settings.computer_use.open_permission")}
                            </UiButton>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ) : null}
              {settings.busy ? (
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} role="status">
                  {t("settings.computer_use.working")}
                </p>
              ) : null}
              {settings.error ? (
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME} role="alert">
                  {t("settings.computer_use.failed")}
                </p>
              ) : null}
            </div>
            {enabled ? (
              <div className="flex flex-wrap items-center justify-end gap-2">
                <UiButton
                  disabled={settings.busy}
                  onClick={() => void settings.onCheckPermissions()}
                  size="xs"
                  variant="text"
                >
                  {t("settings.computer_use.check_permissions")}
                </UiButton>
              </div>
            ) : null}
          </div>
        </div>
      </section>
    </div>
  );
}
