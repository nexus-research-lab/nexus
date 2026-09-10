// INPUT: 主题、语言与本机聊天排版偏好。
// OUTPUT: 即时预览、范围反馈与恢复默认动作；输入法确认不提前提交数字草稿。
// POS: General 外观分区纯视图；不持久化服务端 Preferences。
"use client";

import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { Languages, Palette, RotateCcw } from "lucide-react";
import { useId, useState, type ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { CHAT_TYPOGRAPHY_LIMITS, DEFAULT_CHAT_TYPOGRAPHY, useChatTypography } from "@/shared/theme/chat-typography";
import { defaultTheme, useTheme } from "@/shared/theme/theme-context";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { SettingsFontPicker } from "../components/settings-font-picker";
import { LOCALE_OPTIONS, THEME_OPTIONS } from "../model/settings-options";
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

      <div className={`${SETTINGS_CARD_CLASS_NAME} px-5`}>
        <div className="nexus-chat-feed h-48 overflow-y-auto border-b border-(--divider-subtle-color) py-5">
          <p className={cn("mb-3", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
            {t("settings.appearance.preview")}
          </p>
          <UiMarkdownContent content={t("settings.reading.preview")} />
        </div>
        <AppearanceRow label={t("settings.reading.font")}>
          <SettingsFontPicker value={typography.font} onChange={(font) => setTypography({ font })} />
        </AppearanceRow>
        <AppearanceRow label={t("settings.reading.size")}>
          <AppearanceNumberInput
            label={t("settings.reading.size")}
            value={typography.fontSize}
            min={CHAT_TYPOGRAPHY_LIMITS.fontSize.min} max={CHAT_TYPOGRAPHY_LIMITS.fontSize.max} step={1}
            unit="px"
            onChange={(value) => setTypography({ fontSize: value })}
          />
        </AppearanceRow>
        <AppearanceRow label={t("settings.reading.spacing")}>
          <AppearanceNumberInput
            label={t("settings.reading.spacing")}
            value={typography.lineHeight}
            min={CHAT_TYPOGRAPHY_LIMITS.lineHeight.min} max={CHAT_TYPOGRAPHY_LIMITS.lineHeight.max} step={0.05}
            unit=""
            onChange={(value) => setTypography({ lineHeight: value })}
          />
        </AppearanceRow>

        <div className="flex flex-wrap items-center justify-between gap-3 py-4">
          <p className={getUiTypographyClassName({ role: "supporting", tone: "soft" })}>
            {t("settings.appearance.reset_description")}
          </p>
          <UiButton
            className="shrink-0"
            onClick={resetAppearance}
            size="sm"
            variant="outline"
          >
            <RotateCcw aria-hidden="true" className="h-3.5 w-3.5" />
            {t("settings.appearance.reset")}
          </UiButton>
        </div>
      </div>
    </section>
  );
}

function AppearanceRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid min-h-16 grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)] items-center gap-4 border-b border-(--divider-subtle-color) py-3 text-(--text-strong) last:border-b-0 sm:grid-cols-[minmax(0,1fr)_240px]">
      <span className={getUiTypographyClassName({ role: "control", weight: "medium" })}>
        {label}
      </span>
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
  const { t } = useI18n();
  const hintId = useId();
  const [draft, setDraft] = useState<string | null>(null);
  const [adjusted, setAdjusted] = useState<number | null>(null);
  const numericDraft = draft === null || draft === "" ? null : Number(draft);
  const outOfRange = numericDraft !== null && Number.isFinite(numericDraft) && (numericDraft < min || numericDraft > max);
  const range = t("settings.appearance.value_range", { min, max, unit });
  const hint = outOfRange
    ? t("settings.appearance.range_warning", { min, max, unit })
    : adjusted !== null && adjusted === value
      ? t("settings.appearance.range_adjusted", { min, max, unit, value: adjusted })
      : range;
  return (
    <div>
      <div className="relative">
      <UiInput
        aria-label={label}
        aria-describedby={hintId}
        aria-invalid={outOfRange}
        className="w-full [appearance:textfield] pr-10 tabular-nums [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
        controlSize="lg"
        type="number"
        min={min} max={max} step={step}
        value={draft ?? value}
        onChange={(event) => {
          setAdjusted(null);
          setDraft(event.target.value);
          if (event.target.value !== "" && event.target.validity.valid) onChange(event.target.valueAsNumber);
        }}
        onBlur={(event) => {
          // 输入中允许清空和小数中间态；失焦时才收口范围或恢复原值。
          const entered = event.target.valueAsNumber;
          if (Number.isFinite(entered)) {
            const bounded = Math.min(max, Math.max(min, entered));
            setAdjusted(bounded !== entered ? bounded : null);
            onChange(bounded);
          }
          setDraft(null);
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter" && !isImeKeyboardEvent(event.nativeEvent)) {
            event.currentTarget.blur();
          }
        }}
      />
      {unit && (
        <span className={cn(
          "pointer-events-none absolute inset-y-0 right-3 flex items-center",
          getUiTypographyClassName({ role: "caption", tone: "soft" }),
        )}>
          {unit}
        </span>
      )}
      </div>
      <p id={hintId} role="status" className={cn("mt-1.5", getUiTypographyClassName({ role: "caption", tone: outOfRange ? "warning" : "muted" }))}>
        {hint}
      </p>
    </div>
  );
}
