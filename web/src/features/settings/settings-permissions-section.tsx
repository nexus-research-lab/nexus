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
} from "./settings-panel-ui";

interface SettingsPermissionsSectionProps {
  feedback_message?: string | null;
  on_permission_mode_change: (value: string) => void;
  permission_mode: string;
  preferences_loading: boolean;
}

export function SettingsPermissionsSection({
  feedback_message,
  on_permission_mode_change,
  permission_mode,
  preferences_loading,
}: SettingsPermissionsSectionProps) {
  const { t } = useI18n();
  const selected_permission_mode = AGENT_PERMISSION_MODES.find((mode) => mode.value === permission_mode) ?? AGENT_PERMISSION_MODES[0];

  return (
    <section className="space-y-2.5">
      <div className="flex items-center justify-between gap-3 px-1">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.general.section_permissions")}
        </h2>
        {feedback_message ? (
          <span className="inline-flex items-center gap-1.5 text-[11px] text-(--destructive)">
            {feedback_message}
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
              aria_label={t("settings.general.default_permission_mode")}
              button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
              class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
              disabled={preferences_loading}
              id="default-permission-mode"
              menu_class_name="rounded-[12px]"
              on_change={on_permission_mode_change}
              options={AGENT_PERMISSION_MODES.map((mode) => ({
                value: mode.value,
                label: t(mode.label_key),
              }))}
              placement="top"
              size="xs"
              value={permission_mode}
            />
            <p className="text-[11px] leading-4 text-(--text-soft)">
              {t(selected_permission_mode.description_key)}
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
