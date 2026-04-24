/**
 * # !/usr/bin/env tsx
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：settings-panel.tsx
 * # @Date   ：2026/04/14 23:14
 * # @Author ：leemysw
 * # 2026/04/14 23:14   Create
 * # =====================================================
 */

"use client";

import { Cable, Compass, Languages, Palette, RotateCcw, UserRound } from "lucide-react";
import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/tour-provider";
import { LanguageSwitch } from "@/shared/ui/i18n/language-switch";
import { useTheme } from "@/shared/theme/theme-context";
import { ThemeSwitch } from "@/shared/ui/theme/theme-switch";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { ProviderSettingsPanel } from "./provider-settings-panel";
import { PersonalSettingsPanel } from "./personal-settings-panel";

type SettingsTabKey = "general" | "personal" | "providers";

const SETTINGS_TABS: {
  key: SettingsTabKey;
  label_key: "settings.tabs.general" | "settings.tabs.personal" | "settings.tabs.providers";
  icon: typeof Palette;
}[] = [
  { key: "general", label_key: "settings.tabs.general", icon: Palette },
  { key: "personal", label_key: "settings.tabs.personal", icon: UserRound },
  { key: "providers", label_key: "settings.tabs.providers", icon: Cable },
];

function get_theme_label(
  theme: ReturnType<typeof useTheme>["theme"],
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (theme) {
    case "light":
      return t("theme.light");
    case "dark":
      return t("theme.dark");
    case "sunny":
      return t("theme.sunny");
    case "rain":
      return t("theme.rain");
    default:
      return theme;
  }
}

function get_locale_label(
  locale: ReturnType<typeof useI18n>["locale"],
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (locale) {
    case "zh":
      return t("language.zh");
    case "en":
      return t("language.en");
    default:
      return locale;
  }
}

function GeneralSettingsSection() {
  const { locale, t } = useI18n();
  const { theme } = useTheme();
  const { reset_all_tours } = useOnboardingTour();
  const current_theme_label = useMemo(() => get_theme_label(theme, t), [theme, t]);
  const current_locale_label = useMemo(() => get_locale_label(locale, t), [locale, t]);

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-3 px-1 py-3">
      <section>
        <h2 className="text-[20px] font-semibold tracking-tight text-(--text-strong)">
          {t("settings.general.title")}
        </h2>
      </section>

      <section className="overflow-hidden rounded-[18px] border border-(--divider-subtle-color) bg-(--surface-card-background)">
        <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div className="min-w-0">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <Palette className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                  {t("theme.switch_title")}
                </h3>
              </div>
            </div>
          </div>
          <div className="flex flex-col items-start gap-2 lg:items-end">
            <div className="rounded-full border border-(--divider-subtle-color) bg-(--surface-inset-background) px-2.5 py-1 text-[10px] font-medium text-(--text-default)">
              {current_theme_label}
            </div>
            <ThemeSwitch density="compact" stretch />
          </div>
        </div>

        <div className="mx-3 border-t border-(--divider-subtle-color)" />

        <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div className="min-w-0">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <Languages className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                  {t("language.switch_title")}
                </h3>
              </div>
            </div>
          </div>
          <div className="flex flex-col items-start gap-2 lg:items-end">
            <div className="rounded-full border border-(--divider-subtle-color) bg-(--surface-inset-background) px-2.5 py-1 text-[10px] font-medium text-(--text-default)">
              {current_locale_label}
            </div>
            <LanguageSwitch density="compact" show_icon={false} stretch />
          </div>
        </div>

        <div className="mx-3 border-t border-(--divider-subtle-color)" />

        <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div className="min-w-0">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                <Compass className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                  {t("settings.onboarding_title")}
                </h3>
                <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
                  {t("settings.onboarding_description")}
                </p>
              </div>
            </div>
          </div>
          <div className="flex flex-col items-start gap-2 lg:items-end">
            <button
              className="inline-flex items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-(--surface-inset-background) px-3 py-2 text-[11px] font-semibold text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={reset_all_tours}
              type="button"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              {t("settings.onboarding_action_reset")}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

export function SettingsPanel() {
  const { t } = useI18n();
  const [active_tab, set_active_tab] = useState<SettingsTabKey>("general");
  const active_tab_config = SETTINGS_TABS.find((item) => item.key === active_tab) ?? SETTINGS_TABS[0];
  const ActiveIcon = active_tab_config.icon;

  return (
    <WorkspaceSurfaceScaffold
      body_scrollable
      stable_gutter
      header={(
        <WorkspaceSurfaceHeader
          active_tab={active_tab}
          density="compact"
          leading={<ActiveIcon className="h-4 w-4" />}
          on_change_tab={set_active_tab}
          tabs={SETTINGS_TABS.map((item) => ({
            key: item.key,
            label: t(item.label_key),
            icon: item.icon,
          }))}
          title={t("settings.title")}
        />
      )}
    >
      {active_tab === "general" ? <GeneralSettingsSection /> : null}
      {active_tab === "personal" ? <PersonalSettingsPanel /> : null}
      {active_tab === "providers" ? <ProviderSettingsPanel embedded /> : null}
    </WorkspaceSurfaceScaffold>
  );
}
