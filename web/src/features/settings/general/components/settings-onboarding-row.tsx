"use client";

import { Compass, RotateCcw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";

import {
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";

interface SettingsOnboardingRowProps {
  onReset: () => void;
}

export function SettingsOnboardingRow({ onReset }: SettingsOnboardingRowProps) {
  const { t } = useI18n();

  return (
    <div className={SETTINGS_ROW_CLASS_NAME}>
      <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
        <div className={SETTINGS_ICON_CLASS_NAME}>
          <Compass className="h-3.5 w-3.5" />
        </div>
        <div className="min-w-0">
          <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
            {t("settings.onboarding_title")}
          </h3>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t("settings.onboarding_description")}
          </p>
        </div>
      </div>
      <UiButton
        onClick={onReset}
        size="xs"
        variant="surface"
      >
        <RotateCcw className="h-3 w-3" />
        {t("settings.onboarding_action_reset")}
      </UiButton>
    </div>
  );
}
