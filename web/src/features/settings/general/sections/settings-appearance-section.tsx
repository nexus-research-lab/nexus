"use client";

import { Languages, Palette } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";

import {
  LOCALE_OPTIONS,
  THEME_OPTIONS,
} from "../model/settings-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
} from "../../shared/settings-panel-ui";

export function SettingsAppearanceSection() {
  const { locale, setLocale, t } = useI18n();
  const { setTheme, theme } = useTheme();

  return (
    <section className="space-y-2.5">
      <div className={SETTINGS_CARD_CLASS_NAME}>
        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Palette className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("theme.switch_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.theme_description")}
              </p>
            </div>
          </div>
          <div className="min-w-0">
            <UiSegmentedControl
              density="compact"
              onChange={setTheme}
              options={THEME_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              stretch
              title={t("theme.switch_title")}
              value={theme}
            />
          </div>
        </div>

        <div className="border-t border-(--divider-subtle-color)" />

        <div className={SETTINGS_ROW_CLASS_NAME}>
          <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
            <div className={SETTINGS_ICON_CLASS_NAME}>
              <Languages className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0">
              <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                {t("language.switch_title")}
              </h3>
              <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                {t("settings.general.language_description")}
              </p>
            </div>
          </div>
          <div className="min-w-0">
            <UiSegmentedControl
              density="compact"
              onChange={setLocale}
              options={LOCALE_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              stretch
              title={t("language.switch_title")}
              value={locale}
            />
          </div>
        </div>
      </div>
    </section>
  );
}
