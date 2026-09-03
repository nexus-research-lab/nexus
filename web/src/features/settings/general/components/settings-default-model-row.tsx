/**
 * INPUT: 一个默认模型角色的选项、当前值和保存状态。
 * OUTPUT: 不承载异常文案的模型选择行。
 * POS: 默认模型纯展示组件；目录失败由分区级资源状态统一展示。
 */
"use client";

import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import type { UiSelectMenuOption } from "@/shared/ui/menu/select-menu-model";

import type { DefaultModelPreferenceRole } from "../model/default-model-preferences-model";
import {
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_SELECT_BUTTON_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";

const SETTINGS_DEFAULT_MODEL_ROW_CLASS_NAME = "grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_300px] md:items-center";

interface SettingsDefaultModelRowProps {
  disabled: boolean;
  descriptionKey: TranslationKey;
  emptyPlaceholderKey: TranslationKey;
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
    <div className={SETTINGS_DEFAULT_MODEL_ROW_CLASS_NAME}>
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
          leading={savingRole === modelCategory
            ? <Loader2 className={getUiSpinnerClassName({ size: "xs" })} />
            : null}
          onChange={(nextValue) => onChange(nextValue, modelCategory)}
          options={options}
          placeholder={providerOptionsLoading
            ? t("settings.general.default_model_loading")
            : t(emptyPlaceholderKey)}
          size="xs"
          value={value}
        />
      </div>
    </div>
  );
}
