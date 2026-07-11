"use client";

import { ShieldCheck } from "lucide-react";

import {
  AGENT_PERMISSION_MODES,
} from "@/features/agents/options/agent-options-constants";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSelectMenu } from "@/shared/ui/select-menu";

import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_SELECT_BUTTON_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";

interface SettingsPermissionsSectionProps {
  feedbackMessage?: string | null;
  onPermissionModeChange: (value: string) => void;
  permissionMode: string;
  preferencesLoading: boolean;
  preferencesSaving: boolean;
}

export function SettingsPermissionsSection({
  feedbackMessage,
  onPermissionModeChange,
  permissionMode,
  preferencesLoading,
  preferencesSaving,
}: SettingsPermissionsSectionProps) {
  const { t } = useI18n();
  const selectedPermissionMode = AGENT_PERMISSION_MODES.find((mode) => mode.value === permissionMode) ?? AGENT_PERMISSION_MODES[0];

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.general.section_permissions")}
        </h2>
        {feedbackMessage ? (
          <span className="inline-flex items-center gap-1.5 text-[11px] text-(--destructive)">
            {feedbackMessage}
          </span>
        ) : null}
      </div>
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
            <label className={SETTINGS_CONTROL_LABEL_CLASS_NAME} htmlFor="default-permission-mode">
              {t("settings.general.default_permission_mode")}
            </label>
            <UiSelectMenu
              ariaLabel={t("settings.general.default_permission_mode")}
              buttonClassName={SETTINGS_SELECT_BUTTON_CLASS_NAME}
              className={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
              disabled={preferencesLoading || preferencesSaving}
              id="default-permission-mode"
              menuClassName="rounded-[12px]"
              onChange={onPermissionModeChange}
              options={AGENT_PERMISSION_MODES.map((mode) => ({
                value: mode.value,
                label: t(mode.labelKey),
              }))}
              placement="top"
              size="xs"
              value={permissionMode}
            />
            <p className="text-[11px] leading-4 text-(--text-soft)">
              {t(selectedPermissionMode.descriptionKey)}
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
