"use client";

import { Languages, Palette } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useTheme } from "@/shared/theme/theme-context";

import {
  LOCALE_OPTIONS,
  THEME_OPTIONS,
} from "./settings-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsSegmentedControl,
} from "./settings-panel-ui";

export function SettingsAppearanceSection() {
  const { locale, set_locale, t } = useI18n();
  const { set_theme, theme } = useTheme();

  return (
    <section className="space-y-2.5">
      <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
        {t("settings.general.section_appearance")}
      </h2>
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
            <SettingsSegmentedControl
              aria_label={t("theme.switch_title")}
              on_change={set_theme}
              options={THEME_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.label_key),
              }))}
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
            <SettingsSegmentedControl
              aria_label={t("language.switch_title")}
              on_change={set_locale}
              options={LOCALE_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.label_key),
              }))}
              value={locale}
            />
          </div>
        </div>
      </div>
    </section>
  );
}
