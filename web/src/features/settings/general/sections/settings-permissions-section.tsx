/**
 * INPUT: 默认权限草稿、Preferences 可靠性反馈和恢复动作。
 * OUTPUT: 权限选择器及完整的设置结果提示。
 * POS: General 权限分区视图；不另建单行错误投影。
 */
"use client";

import { ShieldCheck } from "lucide-react";

import {
  AGENT_PERMISSION_MODES,
} from "@/lib/agent-options";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { PreferencesReliabilityNotice } from "../components/preferences-reliability-notice";
import type {
  PreferenceFeedback,
  PreferenceRecoveryControls,
} from "../model/settings-preferences-model";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SELECT_BUTTON_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";

interface SettingsPermissionsSectionProps {
  onPermissionModeChange: (value: string) => void;
  permissionMode: string;
  preferencesLoading: boolean;
  preferencesSaving: boolean;
  preferencesFeedback: PreferenceFeedback | null;
  preferencesRecovery: PreferenceRecoveryControls;
}

export function SettingsPermissionsSection({
  onPermissionModeChange,
  permissionMode,
  preferencesLoading,
  preferencesSaving,
  preferencesFeedback,
  preferencesRecovery,
}: SettingsPermissionsSectionProps) {
  const { t } = useI18n();
  const selectedPermissionMode = AGENT_PERMISSION_MODES.find((mode) => mode.value === permissionMode) ?? AGENT_PERMISSION_MODES[0];

  return (
    <section className="space-y-2.5">
      <PreferencesReliabilityNotice
        feedback={preferencesFeedback}
        recovery={preferencesRecovery}
      />
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <ShieldCheck className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("settings.general.agent_defaults_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.agent_defaults_description")}
              </p>
            </div>
          </div>
          <div className="relative flex min-w-0 flex-col gap-1.5">
            <UiSelectMenu
              ariaLabel={t("settings.general.default_permission_mode")}
              buttonClassName={SETTINGS_SELECT_BUTTON_CLASS_NAME}
              className={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
              disabled={preferencesLoading || preferencesSaving}
              id="default-permission-mode"
              onChange={onPermissionModeChange}
              options={AGENT_PERMISSION_MODES.map((mode) => ({
                value: mode.value,
                label: t(mode.labelKey),
              }))}
              placement="bottom"
              size="xs"
              value={permissionMode}
            />
            <p className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
              {t(selectedPermissionMode.descriptionKey)}
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
