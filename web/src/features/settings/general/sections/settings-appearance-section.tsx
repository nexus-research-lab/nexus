"use client";

import { Languages, Palette, RotateCcw } from "lucide-react";
import { useState, type ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { DEFAULT_CHAT_TYPOGRAPHY, useChatTypography } from "@/shared/theme/chat-typography";
import { defaultTheme, useTheme } from "@/shared/theme/theme-context";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";

import { SettingsFontPicker } from "../components/settings-font-picker";
import { LOCALE_OPTIONS, THEME_OPTIONS } from "../model/settings-options";
import {
  SETTINGS_CARD_CLASS_NAME,
  SETTINGS_ICON_CLASS_NAME,
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_ROW_CLASS_NAME,
  SETTINGS_TEXT_ROW_CLASS_NAME,
  SettingsSegmentedControl,
} from "../../shared/settings-panel-ui";

export function SettingsAppearanceSection() {
  const { locale, setLocale, t } = useI18n();
  const { setTheme, theme } = useTheme();
  const { typography, setTypography } = useChatTypography();

  function resetAppearance() {
    setTheme(defaultTheme());
    setTypography(DEFAULT_CHAT_TYPOGRAPHY);
  }

  return (
    <section className="space-y-4">
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
              ariaLabel={t("theme.switch_title")}
              onChange={setTheme}
              options={THEME_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
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
              ariaLabel={t("language.switch_title")}
              onChange={setLocale}
              options={LOCALE_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              value={locale}
            />
          </div>
        </div>
      </div>

      <div className={`${SETTINGS_CARD_CLASS_NAME} px-5`}>
        <div className="nexus-chat-feed h-48 overflow-y-auto border-b border-(--divider-subtle-color) py-5">
          <p className="mb-3 text-xs text-(--text-soft)">{t("settings.appearance.preview")}</p>
          <UiMarkdownContent content={t("settings.reading.preview")} />
        </div>
        <AppearanceRow label={t("settings.reading.font")}>
          <SettingsFontPicker value={typography.font} onChange={(font) => setTypography({ font })} />
        </AppearanceRow>
        <AppearanceRow label={t("settings.reading.size")}>
          <AppearanceNumberInput
            label={t("settings.reading.size")}
            value={typography.fontSize}
            min={14} max={22} step={1}
            unit="px"
            onChange={(value) => setTypography({ fontSize: value })}
          />
        </AppearanceRow>
        <AppearanceRow label={t("settings.reading.spacing")}>
          <AppearanceNumberInput
            label={t("settings.reading.spacing")}
            value={typography.lineHeight}
            min={1.4} max={2} step={0.05}
            unit=""
            onChange={(value) => setTypography({ lineHeight: value })}
          />
        </AppearanceRow>

        <div className="flex flex-wrap items-center justify-between gap-3 py-4">
          <p className="text-compact text-(--text-soft)">{t("settings.appearance.reset_description")}</p>
          <button
            type="button"
            className="inline-flex h-9 shrink-0 items-center gap-2 rounded-lg border border-(--divider-subtle-color) px-3 text-compact text-(--text-strong) hover:bg-(--surface-interactive-hover-background)"
            onClick={resetAppearance}
          >
            <RotateCcw aria-hidden="true" className="h-3.5 w-3.5" />
            {t("settings.appearance.reset")}
          </button>
        </div>
      </div>
    </section>
  );
}

function AppearanceRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid min-h-16 grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)] items-center gap-4 border-b border-(--divider-subtle-color) py-3 text-(--text-strong) last:border-b-0 sm:grid-cols-[minmax(0,1fr)_240px]">
      <span className="text-base font-medium">{label}</span>
      <div className="min-w-0">{children}</div>
    </div>
  );
}


function AppearanceNumberInput({ label, value, min, max, step, unit, onChange }: {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  unit: string;
  onChange: (value: number) => void;
}) {
  const [draft, setDraft] = useState<string | null>(null);
  return (
    <div className="relative">
      <input
        aria-label={label}
        className="h-10 w-full [appearance:textfield] rounded-lg border border-(--divider-subtle-color) bg-transparent px-3 pr-10 text-sm tabular-nums [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
        type="number"
        min={min} max={max} step={step}
        value={draft ?? value}
        onChange={(event) => {
          setDraft(event.target.value);
          if (event.target.value !== "" && event.target.validity.valid) onChange(event.target.valueAsNumber);
        }}
        onBlur={(event) => {
          // 输入中允许清空和小数中间态；失焦时才收口范围或恢复原值。
          if (Number.isFinite(event.target.valueAsNumber)) onChange(event.target.valueAsNumber);
          setDraft(null);
        }}
        onKeyDown={(event) => { if (event.key === "Enter") event.currentTarget.blur(); }}
      />
      {unit && <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-(--text-soft)">{unit}</span>}
    </div>
  );
}
