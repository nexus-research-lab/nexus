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
  model_category: DefaultModelPreferenceRole;
  saving_role: DefaultModelPreferenceRole | null;
  title_key: TranslationKey;
  value: string;
}

export function SettingsDefaultModelRow({
  description_key: descriptionKey,
  empty_placeholder_key: emptyPlaceholderKey,
  feedback_message: feedbackMessage,
  icon,
  on_change: onChange,
  options,
  provider_options_loading: providerOptionsLoading,
  model_category: modelCategory,
  saving_role: savingRole,
  title_key: titleKey,
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
            {t(titleKey)}
          </h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t(descriptionKey)}
          </p>
        </div>
      </div>
      <div className="flex min-w-0 flex-col gap-1.5">
        <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
          {t("settings.general.default_model_label")}
        </span>
        <UiSelectMenu
          aria_label={t(titleKey)}
          button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
          class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
          disabled={providerOptionsLoading || !!savingRole || options.length === 0}
          leading={savingRole === modelCategory ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
          menu_class_name="min-w-[260px]"
          on_change={(nextValue) => onChange(nextValue, modelCategory)}
          options={options}
          placeholder={providerOptionsLoading
            ? t("settings.general.default_model_loading")
            : t(emptyPlaceholderKey)}
          size="xs"
          value={value}
        />
        {feedbackMessage ? (
          <span className="truncate text-[11px] text-(--text-soft)">
            {feedbackMessage}
          </span>
        ) : null}
      </div>
    </div>
  );
}
