"use client";

import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiSelectMenu, type UiSelectMenuOption } from "@/shared/ui/select-menu";

import type { DefaultModelPreferenceRole } from "./settings-preferences-model";
import {
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SELECT_BUTTON_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "./settings-panel-ui";

interface SettingsDefaultModelRowProps {
  description_key: TranslationKey;
  empty_placeholder_key: TranslationKey;
  feedback_message?: string | null;
  icon: ReactNode;
  on_change: (value: string, role: DefaultModelPreferenceRole) => void;
  options: UiSelectMenuOption[];
  provider_options_loading: boolean;
  role: DefaultModelPreferenceRole;
  saving_role: DefaultModelPreferenceRole | null;
  title_key: TranslationKey;
  value: string;
}

export function SettingsDefaultModelRow({
  description_key,
  empty_placeholder_key,
  feedback_message,
  icon,
  on_change,
  options,
  provider_options_loading,
  role,
  saving_role,
  title_key,
  value,
}: SettingsDefaultModelRowProps) {
  const { t } = useI18n();

  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          {icon}
        </div>
        <div className="min-w-0">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t(title_key)}
          </h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t(description_key)}
          </p>
        </div>
      </div>
      <div className="flex min-w-0 flex-col gap-1.5">
        <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
          {t("settings.general.default_model_label")}
        </span>
        <UiSelectMenu
          aria_label={t(title_key)}
          button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
          class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
          disabled={provider_options_loading || !!saving_role || options.length === 0}
          leading={saving_role === role ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
          menu_class_name="min-w-[260px]"
          on_change={(next_value) => on_change(next_value, role)}
          options={options}
          placeholder={provider_options_loading
            ? t("settings.general.default_model_loading")
            : t(empty_placeholder_key)}
          size="xs"
          value={value}
        />
        {feedback_message ? (
          <span className="truncate text-[11px] text-(--text-soft)">
            {feedback_message}
          </span>
        ) : null}
      </div>
    </div>
  );
}
