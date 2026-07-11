"use client";

import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiSelectMenu, type UiSelectMenuOption } from "@/shared/ui/select-menu";

import type { DefaultModelPreferenceRole } from "../model/settings-preferences-model";
import {
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SELECT_BUTTON_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";

interface SettingsDefaultModelRowProps {
  disabled: boolean;
  descriptionKey: TranslationKey;
  emptyPlaceholderKey: TranslationKey;
  feedbackMessage?: string | null;
  icon: ReactNode;
  onChange: (value: string, role: DefaultModelPreferenceRole) => void;
  options: UiSelectMenuOption[];
  providerOptionsLoading: boolean;
  modelCategory: DefaultModelPreferenceRole;
  savingRole: DefaultModelPreferenceRole | null;
  titleKey: TranslationKey;
  value: string;
}

export function SettingsDefaultModelRow({
  disabled,
  descriptionKey,
  emptyPlaceholderKey,
  feedbackMessage,
  icon,
  onChange,
  options,
  providerOptionsLoading,
  modelCategory,
  savingRole,
  titleKey,
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
          ariaLabel={t(titleKey)}
          buttonClassName={SETTINGS_SELECT_BUTTON_CLASS_NAME}
          className={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
          disabled={
            disabled
            || providerOptionsLoading
            || Boolean(savingRole)
            || options.length === 0
          }
          leading={savingRole === modelCategory ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
          menuClassName="min-w-[260px]"
          onChange={(nextValue) => onChange(nextValue, modelCategory)}
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
