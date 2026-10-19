/**
 * INPUT: Host Computer Use installation status and permission actions.
 * OUTPUT: Dedicated Computer Use settings page with one clear enable switch.
 * POS: Settings domain for local computer control; it is separate from Runtime and Browser.
 */
"use client";

import { MousePointer2 } from "lucide-react";

import { SETTINGS_CONTENT_BODY_CLASS_NAME } from "@/features/settings/shared/settings-panel-ui";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SettingsToggleRow,
} from "../shared/settings-panel-ui";
import { useComputerUseSettingsController } from "./use-computer-use-settings-controller";

export function ComputerUseSettingsSection() {
  const { t } = useI18n();
  const settings = useComputerUseSettingsController();
  const installed = settings.status?.installed === true;
  const enabled = settings.status?.enabled === true;
  const permissions = settings.status?.permissions;
  const permissionsNeedAction = enabled && (
    permissions == null
    || permissions.screen_capture !== "granted"
    || permissions.accessibility !== "granted"
    || permissions.input_control !== "granted"
  );

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
            {permissionsNeedAction ? (
              <div className="flex flex-wrap items-center justify-end gap-2">
                <UiButton
                  disabled={settings.busy}
                  onClick={() => void settings.onOpenPermissions()}
                  size="xs"
                  variant="text"
                >
                  {t("settings.computer_use.open_permissions")}
                </UiButton>
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
