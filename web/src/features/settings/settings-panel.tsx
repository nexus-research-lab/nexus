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

import {
  Cable,
  ChevronDown,
  Compass,
  Languages,
  Loader2,
  MessageSquareText,
  Palette,
  RotateCcw,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { get_user_preferences_api, update_user_preferences_api } from "@/lib/api/settings-preferences-api";
import { cn } from "@/lib/utils";
import {
  get_user_preferences,
  set_user_preferences,
} from "@/config/options";
import {
  AGENT_PERMISSION_MODES,
} from "@/features/agents/options/agent-options-constants";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/tour-provider";
import { type Theme, useTheme } from "@/shared/theme/theme-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { AgentConversationDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { UserPreferences } from "@/types/settings/preferences";
import type { Locale } from "@/shared/i18n/messages";

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

const DELIVERY_POLICY_OPTIONS: ReadonlyArray<{
  value: AgentConversationDeliveryPolicy;
  label_key: "settings.general.default_delivery_queue" | "settings.general.default_delivery_guide" | "settings.general.default_delivery_interrupt";
}> = [
  { value: "queue", label_key: "settings.general.default_delivery_queue" },
  { value: "guide", label_key: "settings.general.default_delivery_guide" },
  { value: "interrupt", label_key: "settings.general.default_delivery_interrupt" },
];

const THEME_OPTIONS: ReadonlyArray<{
  value: Theme;
  label_key: "theme.light" | "theme.dark" | "theme.sunny" | "theme.rain";
}> = [
  { value: "light", label_key: "theme.light" },
  { value: "dark", label_key: "theme.dark" },
  { value: "sunny", label_key: "theme.sunny" },
  { value: "rain", label_key: "theme.rain" },
];

const LOCALE_OPTIONS: ReadonlyArray<{
  value: Locale;
  label_key: "language.zh" | "language.en";
}> = [
  { value: "zh", label_key: "language.zh" },
  { value: "en", label_key: "language.en" },
];

interface PreferenceFeedback {
  message: string;
}

const SETTINGS_SECTION_TITLE_CLASS_NAME = "px-1 text-[17px] font-semibold tracking-tight text-(--text-strong)";
const SETTINGS_CARD_CLASS_NAME = "overflow-hidden rounded-[18px] border border-(--divider-subtle-color) bg-(--surface-card-background)";
const SETTINGS_ROW_CLASS_NAME = "grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(180px,220px)] md:items-center";
const SETTINGS_TEXT_ROW_CLASS_NAME = "flex min-w-0 items-start gap-3";
const SETTINGS_ICON_CLASS_NAME = "flex h-7 w-7 shrink-0 items-center justify-center rounded-[14px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary";
const SETTINGS_ITEM_TITLE_CLASS_NAME = "text-[14px] font-semibold tracking-tight text-(--text-strong)";
const SETTINGS_ITEM_DESCRIPTION_CLASS_NAME = "mt-1 max-w-[520px] text-[12px] leading-5 text-(--text-soft)";
const SETTINGS_CONTROL_LABEL_CLASS_NAME = "text-[11px] font-medium text-(--text-soft)";
const SETTINGS_CONTROL_HEIGHT_CLASS_NAME = "h-7";
const SETTINGS_CONTROL_TEXT_CLASS_NAME = "text-[11px] font-semibold leading-none";
const SETTINGS_SELECT_CLASS_NAME = `${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} w-full appearance-none rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-inset-background) px-2.5 pr-7 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-strong) outline-none transition-colors focus:border-(--surface-interactive-active-border) disabled:opacity-(--disabled-opacity)`;

interface SettingsSegmentedControlOption<T extends string> {
  label: string;
  value: T;
}

function SettingsSegmentedControl<T extends string>({
  aria_label,
  disabled,
  on_change,
  options,
  value,
}: {
  aria_label: string;
  disabled?: boolean;
  on_change: (value: T) => void;
  options: ReadonlyArray<SettingsSegmentedControlOption<T>>;
  value: T;
}) {
  return (
    <div
      aria-label={aria_label}
      className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex w-full items-center rounded-xl border border-(--divider-subtle-color) bg-(--surface-inset-background) p-0.5`}
      role="group"
    >
      {options.map((option) => {
        const active = value === option.value;
        return (
          <button
            key={option.value}
            aria-pressed={active}
            className={cn(
              `inline-flex h-6 min-w-0 flex-1 items-center justify-center rounded-[9px] px-2 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} transition-colors`,
              active
                ? "bg-(--surface-interactive-active-background) text-(--text-strong) shadow-sm"
                : "text-(--text-soft) hover:text-(--text-default)",
            )}
            disabled={disabled}
            onClick={() => on_change(option.value)}
            type="button"
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

function normalize_preferences(preferences: UserPreferences | null): UserPreferences {
  const fallback = get_user_preferences();
  return {
    chat_default_delivery_policy:
      preferences?.chat_default_delivery_policy ?? fallback.chat_default_delivery_policy,
    default_agent_options: {
      ...fallback.default_agent_options,
      ...(preferences?.default_agent_options ?? {}),
      allowed_tools: [
        ...(preferences?.default_agent_options?.allowed_tools ??
          fallback.default_agent_options.allowed_tools ??
          []),
      ],
      disallowed_tools: [
        ...(preferences?.default_agent_options?.disallowed_tools ??
          fallback.default_agent_options.disallowed_tools ??
          []),
      ],
      setting_sources: [
        ...(preferences?.default_agent_options?.setting_sources ??
          fallback.default_agent_options.setting_sources ??
          ["project"]),
      ],
    },
    updated_at: preferences?.updated_at,
  };
}

function GeneralSettingsSection() {
  const { locale, set_locale, t } = useI18n();
  const { set_theme, theme } = useTheme();
  const { reset_all_tours } = useOnboardingTour();
  const [preferences, set_preferences] = useState<UserPreferences>(() => normalize_preferences(null));
  const [preferences_loading, set_preferences_loading] = useState(true);
  const [preferences_saving, set_preferences_saving] = useState(false);
  const [preference_feedback, set_preference_feedback] = useState<PreferenceFeedback | null>(null);
  const preferences_ref = useRef(preferences);
  const last_saved_preferences_ref = useRef<UserPreferences | null>(null);
  const save_sequence_ref = useRef(0);
  const permission_mode = preferences.default_agent_options.permission_mode ?? "bypassPermissions";
  const selected_permission_mode = AGENT_PERMISSION_MODES.find((mode) => mode.value === permission_mode) ?? AGENT_PERMISSION_MODES[0];

  useEffect(() => {
    let cancelled = false;
    const load_preferences = async () => {
      try {
        set_preferences_loading(true);
        const result = await get_user_preferences_api();
        if (cancelled) {
          return;
        }
        const normalized = normalize_preferences(result);
        set_user_preferences(normalized);
        set_preferences(normalized);
        preferences_ref.current = normalized;
        last_saved_preferences_ref.current = normalized;
        set_preference_feedback(null);
      } catch (error) {
        if (!cancelled) {
          set_preference_feedback({
            message: error instanceof Error ? error.message : t("settings.general.preferences_load_failed"),
          });
        }
      } finally {
        if (!cancelled) {
          set_preferences_loading(false);
        }
      }
    };
    void load_preferences();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const persist_preferences = useCallback(async (next_preferences: UserPreferences) => {
    const sequence = save_sequence_ref.current + 1;
    save_sequence_ref.current = sequence;
    const normalized = normalize_preferences(next_preferences);

    preferences_ref.current = normalized;
    set_preferences(normalized);
    set_user_preferences(normalized);
    set_preference_feedback(null);
    set_preferences_saving(true);

    try {
      const result = await update_user_preferences_api({
        chat_default_delivery_policy: normalized.chat_default_delivery_policy,
        default_agent_options: normalized.default_agent_options,
      });
      if (save_sequence_ref.current !== sequence) {
        return;
      }
      const saved = normalize_preferences(result);
      preferences_ref.current = saved;
      last_saved_preferences_ref.current = saved;
      set_user_preferences(saved);
      set_preferences(saved);
    } catch (error) {
      if (save_sequence_ref.current !== sequence) {
        return;
      }
      const fallback = last_saved_preferences_ref.current;
      if (fallback) {
        preferences_ref.current = fallback;
        set_user_preferences(fallback);
        set_preferences(fallback);
      }
      set_preference_feedback({
        message: error instanceof Error ? error.message : t("settings.general.preferences_save_failed"),
      });
    } finally {
      if (save_sequence_ref.current === sequence) {
        set_preferences_saving(false);
      }
    }
  }, [t]);

  const handle_delivery_policy_change = useCallback((value: AgentConversationDeliveryPolicy) => {
    const current_preferences = preferences_ref.current;
    void persist_preferences({
      ...current_preferences,
      chat_default_delivery_policy: value,
    });
  }, [persist_preferences]);

  const handle_permission_mode_change = useCallback((value: string) => {
    const current_preferences = preferences_ref.current;
    void persist_preferences({
      ...current_preferences,
      default_agent_options: {
        ...current_preferences.default_agent_options,
        permission_mode: value,
      },
    });
  }, [persist_preferences]);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 px-1 py-3">
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

      <section className="space-y-2.5">
        <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
          {t("settings.general.section_general")}
        </h2>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <MessageSquareText className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.general.runtime_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.runtime_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.general.default_delivery")}
              </span>
              <SettingsSegmentedControl
                aria_label={t("settings.general.default_delivery")}
                disabled={preferences_loading}
                on_change={handle_delivery_policy_change}
                options={DELIVERY_POLICY_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.label_key),
                }))}
                value={preferences.chat_default_delivery_policy}
              />
            </div>
          </div>

          <div className="border-t border-(--divider-subtle-color)" />

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
            <button
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-(--surface-inset-background) px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)`}
              onClick={reset_all_tours}
              type="button"
            >
              <RotateCcw className="h-3 w-3" />
              {t("settings.onboarding_action_reset")}
            </button>
          </div>
        </div>
      </section>

      <section className="space-y-2.5">
        <div className="flex items-center justify-between gap-3 px-1">
          <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
            {t("settings.general.section_permissions")}
          </h2>
          {preferences_saving || preference_feedback ? (
            <span className={cn(
              "inline-flex items-center gap-1.5 text-[11px]",
              preference_feedback ? "text-(--destructive)" : "text-(--text-soft)",
            )}>
              {preferences_saving ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
              {preference_feedback?.message ?? t("settings.general.preferences_saving")}
            </span>
          ) : null}
        </div>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <ShieldCheck className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.general.agent_defaults_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.agent_defaults_description")}
                </p>
              </div>
            </div>
            <div className="relative flex min-w-0 flex-col gap-1.5">
              <label className={SETTINGS_CONTROL_LABEL_CLASS_NAME} htmlFor="default-permission-mode">
                {t("settings.general.default_permission_mode")}
              </label>
              <select
                id="default-permission-mode"
                className={SETTINGS_SELECT_CLASS_NAME}
                disabled={preferences_loading}
                onChange={(event) => handle_permission_mode_change(event.target.value)}
                value={permission_mode}
              >
                {AGENT_PERMISSION_MODES.map((mode) => (
                  <option key={mode.value} value={mode.value}>
                    {t(mode.label_key)}
                  </option>
                ))}
              </select>
              <ChevronDown className="pointer-events-none absolute right-2.5 top-[28px] h-3 w-3 text-(--text-soft)" />
              <p className="text-[11px] leading-4 text-(--text-soft)">
                {t(selected_permission_mode.description_key)}
              </p>
            </div>
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
