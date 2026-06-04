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
  ArrowLeft,
  Cable,
  Compass,
  Download,
  ExternalLink,
  Image,
  Languages,
  Loader2,
  MessageSquareText,
  MonitorCog,
  PackageOpen,
  Palette,
  RotateCcw,
  ShieldCheck,
  Sparkles,
  Terminal,
  UserRound,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import { download_nxs_runtime_api, get_nxs_runtime_status_api } from "@/lib/api/runtime-api";
import { get_user_preferences_api, update_user_preferences_api } from "@/lib/api/settings-preferences-api";
import {
  get_system_version_api,
  type SystemVersionInfo,
} from "@/lib/api/system-api";
import {
  export_desktop_logs,
  get_desktop_app_version,
  is_desktop_bridge_available,
  open_desktop_route,
  type DesktopAppVersion,
} from "@/lib/desktop-bridge";
import { cn } from "@/lib/utils";
import {
  get_user_preferences,
  set_default_agent_model,
  set_default_agent_provider,
  set_user_preferences,
} from "@/config/options";
import {
  AGENT_PERMISSION_MODES,
  DEFAULT_AGENT_PERMISSION_MODE,
} from "@/features/agents/options/agent-options-constants";
import {
  list_provider_options_api,
} from "@/lib/api/provider-config-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import { type Theme, useTheme } from "@/shared/theme/theme-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { ProviderOption } from "@/types/capability/provider";
import type { AgentRuntimeKind, NXSRuntimeStatus, UserPreferences } from "@/types/settings/preferences";
import type { Locale } from "@/shared/i18n/messages";

import { ProviderSettingsPanel } from "./provider-settings-panel";
import { PersonalSettingsPanel } from "./personal-settings-panel";

type SettingsTabKey = "general" | "personal" | "providers";
type DefaultModelPreferenceRole = "agent_runtime" | "image_generation" | "background_task";

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
  value: AgentConversationDefaultDeliveryPolicy;
  label_key: "settings.general.default_delivery_queue" | "settings.general.default_delivery_interrupt";
}> = [
  { value: "queue", label_key: "settings.general.default_delivery_queue" },
  { value: "interrupt", label_key: "settings.general.default_delivery_interrupt" },
];

const AGENT_RUNTIME_KIND_OPTIONS: ReadonlyArray<{
  value: AgentRuntimeKind;
  label_key: "settings.general.runtime_claude" | "settings.general.runtime_nxs";
}> = [
  { value: "claude", label_key: "settings.general.runtime_claude" },
  { value: "nxs", label_key: "settings.general.runtime_nxs" },
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
const SETTINGS_CARD_CLASS_NAME = "overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent";
const SETTINGS_ROW_CLASS_NAME = "grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(180px,220px)] md:items-center";
const SETTINGS_TEXT_ROW_CLASS_NAME = "flex min-w-0 items-start gap-3";
const SETTINGS_ICON_CLASS_NAME = "flex h-7 w-7 shrink-0 items-center justify-center rounded-[14px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary";
const SETTINGS_ITEM_TITLE_CLASS_NAME = "text-[14px] font-semibold tracking-tight text-(--text-strong)";
const SETTINGS_ITEM_DESCRIPTION_CLASS_NAME = "mt-1 max-w-[520px] text-[12px] leading-5 text-(--text-soft)";
const SETTINGS_CONTROL_LABEL_CLASS_NAME = "text-[11px] font-medium text-(--text-soft)";
const SETTINGS_CONTROL_HEIGHT_CLASS_NAME = "h-7";
const SETTINGS_CONTROL_TEXT_CLASS_NAME = "text-[11px] font-semibold leading-none";
const SETTINGS_SELECT_BUTTON_CLASS_NAME = `${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} w-full rounded-[10px] border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-strong) shadow-none hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background) focus-visible:ring-0`;
const DEFAULT_RELEASE_PAGE_URL = "https://github.com/nexus-research-lab/nexus/releases/latest";

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
      className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex w-full items-center rounded-xl border border-(--divider-subtle-color) bg-transparent p-0.5`}
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
    agent_runtime_kind: normalize_agent_runtime_kind(
      preferences?.agent_runtime_kind ?? fallback.agent_runtime_kind,
    ),
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
    default_image_model_selection: normalize_model_selection_preference(
      preferences?.default_image_model_selection ?? fallback.default_image_model_selection,
    ),
    default_background_model_selection: normalize_model_selection_preference(
      preferences?.default_background_model_selection ?? fallback.default_background_model_selection,
    ),
    updated_at: preferences?.updated_at,
  };
}

function normalize_agent_runtime_kind(value?: string | null): AgentRuntimeKind {
  return value?.trim() === "nxs" ? "nxs" : "claude";
}

function normalize_model_selection_preference(
  selection: UserPreferences["default_image_model_selection"],
): UserPreferences["default_image_model_selection"] {
  const provider = selection?.provider?.trim();
  const model = selection?.model?.trim();
  if (!provider || !model) {
    return undefined;
  }
  return { provider, model };
}

function encode_default_model_value(provider: string, model: string): string {
  return JSON.stringify([provider, model]);
}

function decode_default_model_value(value: string): { provider: string; model: string } | null {
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed) || parsed.length !== 2) {
      return null;
    }
    const [provider, model] = parsed;
    if (typeof provider !== "string" || typeof model !== "string") {
      return null;
    }
    const normalized_provider = provider.trim();
    const normalized_model = model.trim();
    if (!normalized_provider || !normalized_model) {
      return null;
    }
    return { provider: normalized_provider, model: normalized_model };
  } catch {
    return null;
  }
}

function encode_optional_model_selection(
  provider?: string | null,
  model?: string | null,
): string {
  const normalized_provider = provider?.trim();
  const normalized_model = model?.trim();
  if (!normalized_provider || !normalized_model) {
    return "";
  }
  return encode_default_model_value(normalized_provider, normalized_model);
}

function GeneralSettingsSection() {
  const { locale, set_locale, t } = useI18n();
  const { set_theme, theme } = useTheme();
  const { reset_all_tours } = useOnboardingTour();
  const [preferences, set_preferences] = useState<UserPreferences>(() => normalize_preferences(null));
  const [preferences_loading, set_preferences_loading] = useState(true);
  const [preferences_saving, set_preferences_saving] = useState(false);
  const [preference_feedback, set_preference_feedback] = useState<PreferenceFeedback | null>(null);
  const [provider_options, set_provider_options] = useState<ProviderOption[]>([]);
  const [background_provider_options, set_background_provider_options] = useState<ProviderOption[]>([]);
  const [image_provider_options, set_image_provider_options] = useState<ProviderOption[]>([]);
  const [default_model_value, set_default_model_value] = useState("");
  const [default_image_model_value, set_default_image_model_value] = useState("");
  const [default_background_model_value, set_default_background_model_value] = useState("");
  const [provider_options_loading, set_provider_options_loading] = useState(true);
  const [default_model_saving_role, set_default_model_saving_role] = useState<DefaultModelPreferenceRole | null>(null);
  const [default_model_feedback, set_default_model_feedback] = useState<PreferenceFeedback | null>(null);
  const [nxs_runtime_checking, set_nxs_runtime_checking] = useState(false);
  const [nxs_runtime_downloading, set_nxs_runtime_downloading] = useState(false);
  const [nxs_download_prompt_status, set_nxs_download_prompt_status] = useState<NXSRuntimeStatus | null>(null);
  const [system_version, set_system_version] = useState<SystemVersionInfo | null>(null);
  const [system_version_loading, set_system_version_loading] = useState(true);
  const [system_version_feedback, set_system_version_feedback] = useState<PreferenceFeedback | null>(null);
  const preferences_ref = useRef(preferences);
  const last_saved_preferences_ref = useRef<UserPreferences | null>(null);
  const provider_default_selection_ref = useRef({ provider: "", model: "" });
  const image_default_selection_ref = useRef({ provider: "", model: "" });
  const save_sequence_ref = useRef(0);
  const agent_runtime_kind = normalize_agent_runtime_kind(preferences.agent_runtime_kind);
  const permission_mode = preferences.default_agent_options.permission_mode ?? DEFAULT_AGENT_PERMISSION_MODE;
  const selected_permission_mode = AGENT_PERMISSION_MODES.find((mode) => mode.value === permission_mode) ?? AGENT_PERMISSION_MODES[0];
  const [desktop_available] = useState(() => is_desktop_bridge_available());
  const [desktop_version, set_desktop_version] = useState<DesktopAppVersion | null>(null);
  const [desktop_feedback, set_desktop_feedback] = useState<PreferenceFeedback | null>(null);
  const [exporting_logs, set_exporting_logs] = useState(false);

  const load_provider_options = useCallback(async (runtime_kind?: AgentRuntimeKind) => {
    try {
      set_provider_options_loading(true);
      const selected_runtime_kind = runtime_kind ?? normalize_agent_runtime_kind(preferences_ref.current.agent_runtime_kind);
      const result = await list_provider_options_api(selected_runtime_kind);
      set_provider_options(result.items ?? []);
      set_background_provider_options(result.background_items ?? result.items ?? []);
      set_image_provider_options(result.image_items ?? []);
      provider_default_selection_ref.current = {
        provider: result.default_provider?.trim() || "",
        model: result.default_model?.trim() || "",
      };
      image_default_selection_ref.current = {
        provider: result.default_image_provider?.trim() || "",
        model: result.default_image_model?.trim() || "",
      };
      const current_preferences = preferences_ref.current;
      const agent_provider = current_preferences.default_agent_options.provider?.trim()
        || provider_default_selection_ref.current.provider;
      const agent_model = current_preferences.default_agent_options.model?.trim()
        || provider_default_selection_ref.current.model;
      set_default_agent_provider(agent_provider);
      set_default_agent_model(agent_model);
      set_default_model_value(encode_optional_model_selection(agent_provider, agent_model));
      set_default_image_model_value(
        encode_optional_model_selection(
          current_preferences.default_image_model_selection?.provider
            || image_default_selection_ref.current.provider,
          current_preferences.default_image_model_selection?.model
            || image_default_selection_ref.current.model,
        ),
      );
      set_default_background_model_value(
        encode_optional_model_selection(
          current_preferences.default_background_model_selection?.provider,
          current_preferences.default_background_model_selection?.model,
        ),
      );
      set_default_model_feedback(null);
    } catch (error) {
      set_default_model_feedback({
        message: error instanceof Error ? error.message : "默认对话模型加载失败",
      });
    } finally {
      set_provider_options_loading(false);
    }
  }, []);

  useEffect(() => {
    void load_provider_options(agent_runtime_kind);
  }, [agent_runtime_kind, load_provider_options]);

  useEffect(() => {
    let cancelled = false;
    const load_system_version = async () => {
      try {
        set_system_version_loading(true);
        const result = await get_system_version_api();
        if (cancelled) {
          return;
        }
        set_system_version(result);
        set_system_version_feedback(null);
      } catch (error) {
        if (!cancelled) {
          set_system_version_feedback({
            message: error instanceof Error ? error.message : t("settings.system.version_failed"),
          });
        }
      } finally {
        if (!cancelled) {
          set_system_version_loading(false);
        }
      }
    };
    void load_system_version();
    return () => {
      cancelled = true;
    };
  }, [t]);

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
        const agent_provider = normalized.default_agent_options.provider?.trim()
          || provider_default_selection_ref.current.provider;
        const agent_model = normalized.default_agent_options.model?.trim()
          || provider_default_selection_ref.current.model;
        set_default_agent_provider(agent_provider);
        set_default_agent_model(agent_model);
        set_default_model_value(encode_optional_model_selection(agent_provider, agent_model));
        set_default_image_model_value(
          encode_optional_model_selection(
            normalized.default_image_model_selection?.provider
              || image_default_selection_ref.current.provider,
            normalized.default_image_model_selection?.model
              || image_default_selection_ref.current.model,
          ),
        );
        set_default_background_model_value(
          encode_optional_model_selection(
            normalized.default_background_model_selection?.provider,
            normalized.default_background_model_selection?.model,
          ),
        );
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

  useEffect(() => {
    if (!desktop_available) {
      return;
    }
    let cancelled = false;
    const load_version = async () => {
      try {
        const version = await get_desktop_app_version();
        if (!cancelled) {
          set_desktop_version(version);
        }
      } catch (error) {
        if (!cancelled) {
          set_desktop_feedback({
            message: error instanceof Error ? error.message : t("settings.desktop.version_failed"),
          });
        }
      }
    };
    void load_version();
    return () => {
      cancelled = true;
    };
  }, [desktop_available, t]);

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
        agent_runtime_kind: normalized.agent_runtime_kind,
        default_agent_options: normalized.default_agent_options,
        default_image_model_selection: normalized.default_image_model_selection,
        default_background_model_selection: normalized.default_background_model_selection,
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

  const handle_delivery_policy_change = useCallback((value: AgentConversationDefaultDeliveryPolicy) => {
    const current_preferences = preferences_ref.current;
    void persist_preferences({
      ...current_preferences,
      chat_default_delivery_policy: value,
    });
  }, [persist_preferences]);

  const handle_agent_runtime_kind_change = useCallback((value: AgentRuntimeKind) => {
    const current_preferences = preferences_ref.current;
    if (value === normalize_agent_runtime_kind(current_preferences.agent_runtime_kind)) {
      return;
    }
    if (value !== "nxs") {
      set_nxs_download_prompt_status(null);
      void (async () => {
        await persist_preferences({
          ...current_preferences,
          agent_runtime_kind: value,
        });
        await load_provider_options(value);
      })();
      return;
    }
    void (async () => {
      set_nxs_runtime_checking(true);
      set_preference_feedback(null);
      try {
        const status = await get_nxs_runtime_status_api();
        if (status.available) {
          await persist_preferences({
            ...preferences_ref.current,
            agent_runtime_kind: "nxs",
          });
          await load_provider_options("nxs");
          return;
        }
        if (status.can_download) {
          set_nxs_download_prompt_status(status);
          return;
        }
        set_preference_feedback({
          message: status.message || t("settings.general.agent_runtime_nxs_unavailable"),
        });
      } catch (error) {
        set_preference_feedback({
          message: error instanceof Error ? error.message : t("settings.general.agent_runtime_check_failed"),
        });
      } finally {
        set_nxs_runtime_checking(false);
      }
    })();
  }, [load_provider_options, persist_preferences, t]);

  const handle_confirm_nxs_download = useCallback(() => {
    if (nxs_runtime_downloading) {
      return;
    }
    void (async () => {
      set_nxs_runtime_downloading(true);
      set_preference_feedback(null);
      try {
        const status = await download_nxs_runtime_api();
        if (!status.available) {
          set_nxs_download_prompt_status(status);
          set_preference_feedback({
            message: status.message || t("settings.general.agent_runtime_nxs_unavailable"),
          });
          return;
        }
        set_nxs_download_prompt_status(null);
        await persist_preferences({
          ...preferences_ref.current,
          agent_runtime_kind: "nxs",
        });
        await load_provider_options("nxs");
      } catch (error) {
        const message = error instanceof Error ? error.message : t("settings.general.agent_runtime_download_failed");
        set_nxs_download_prompt_status({
          ...(nxs_download_prompt_status ?? {
            available: false,
            can_download: true,
          }),
          message,
        });
        set_preference_feedback({ message });
      } finally {
        set_nxs_runtime_downloading(false);
      }
    })();
  }, [load_provider_options, nxs_download_prompt_status, nxs_runtime_downloading, persist_preferences, t]);

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

  const default_model_options = useMemo(() => provider_options.flatMap((provider) => (
    provider.models.map((model) => {
      const provider_label = provider.display_name || provider.provider;
      const model_label = model.display_name || model.model_id;
      return {
        value: encode_default_model_value(provider.provider, model.model_id),
        label: `${provider_label} / ${model_label}`,
      };
    })
  )), [provider_options]);

  const default_image_model_options = useMemo(() => image_provider_options.flatMap((provider) => (
    provider.models.map((model) => {
      const provider_label = provider.display_name || provider.provider;
      const model_label = model.display_name || model.model_id;
      return {
        value: encode_default_model_value(provider.provider, model.model_id),
        label: `${provider_label} / ${model_label}`,
      };
    })
  )), [image_provider_options]);

  const default_background_model_options = useMemo(() => background_provider_options.flatMap((provider) => (
    provider.models.map((model) => {
      const provider_label = provider.display_name || provider.provider;
      const model_label = model.display_name || model.model_id;
      return {
        value: encode_default_model_value(provider.provider, model.model_id),
        label: `${provider_label} / ${model_label}`,
      };
    })
  )), [background_provider_options]);

  const handle_default_model_change = useCallback((value: string, role: DefaultModelPreferenceRole) => {
    const selection = decode_default_model_value(value);
    if (!selection || default_model_saving_role) {
      return;
    }
    void (async () => {
      set_default_model_saving_role(role);
      set_default_model_feedback(null);
      const previous_value = role === "image_generation"
        ? default_image_model_value
        : role === "background_task"
        ? default_background_model_value
        : default_model_value;
      if (role === "image_generation") {
        set_default_image_model_value(value);
      } else if (role === "background_task") {
        set_default_background_model_value(value);
      } else {
        set_default_model_value(value);
      }
      try {
        const current_preferences = preferences_ref.current;
        const next_preferences = normalize_preferences({
          ...current_preferences,
          default_agent_options: role === "agent_runtime"
            ? {
              ...current_preferences.default_agent_options,
              provider: selection.provider,
              model: selection.model,
            }
            : current_preferences.default_agent_options,
          default_image_model_selection: role === "image_generation"
            ? { provider: selection.provider, model: selection.model }
            : current_preferences.default_image_model_selection,
          default_background_model_selection: role === "background_task"
            ? { provider: selection.provider, model: selection.model }
            : current_preferences.default_background_model_selection,
        });
        preferences_ref.current = next_preferences;
        set_preferences(next_preferences);
        set_user_preferences(next_preferences);
        const result = await update_user_preferences_api({
          chat_default_delivery_policy: next_preferences.chat_default_delivery_policy,
          agent_runtime_kind: next_preferences.agent_runtime_kind,
          default_agent_options: next_preferences.default_agent_options,
          default_image_model_selection: next_preferences.default_image_model_selection,
          default_background_model_selection: next_preferences.default_background_model_selection,
        });
        const saved = normalize_preferences(result);
        preferences_ref.current = saved;
        last_saved_preferences_ref.current = saved;
        set_preferences(saved);
        set_user_preferences(saved);
        if (role === "agent_runtime") {
          set_default_agent_provider(selection.provider);
          set_default_agent_model(selection.model);
        }
      } catch (error) {
        const fallback = last_saved_preferences_ref.current;
        if (fallback) {
          preferences_ref.current = fallback;
          set_preferences(fallback);
          set_user_preferences(fallback);
          if (role === "agent_runtime") {
            set_default_agent_provider(fallback.default_agent_options.provider);
            set_default_agent_model(fallback.default_agent_options.model);
          }
        }
        if (role === "image_generation") {
          set_default_image_model_value(previous_value);
        } else if (role === "background_task") {
          set_default_background_model_value(previous_value);
        } else {
          set_default_model_value(previous_value);
        }
        set_default_model_feedback({
          message: error instanceof Error ? error.message : "默认对话模型保存失败",
        });
      } finally {
        set_default_model_saving_role(null);
      }
    })();
  }, [
    default_background_model_value,
    default_image_model_value,
    default_model_saving_role,
    default_model_value,
  ]);

  const handle_export_logs = useCallback(async () => {
    try {
      set_exporting_logs(true);
      set_desktop_feedback(null);
      const result = await export_desktop_logs();
      if (result.cancelled) {
        return;
      }
      set_desktop_feedback({
        message: result.path
          ? t("settings.desktop.export_logs_success_with_path").replace("{path}", result.path)
          : t("settings.desktop.export_logs_success"),
      });
    } catch (error) {
      set_desktop_feedback({
        message: error instanceof Error ? error.message : t("settings.desktop.export_logs_failed"),
      });
    } finally {
      set_exporting_logs(false);
    }
  }, [t]);

  const release_page_url = system_version?.release_url || DEFAULT_RELEASE_PAGE_URL;
  const system_version_description = system_version
    ? t("settings.system.version_value")
      .replace("{version}", system_version.version)
      .replace("{target}", system_version.target)
    : system_version_loading
      ? t("settings.system.version_loading")
      : t("settings.system.version_unavailable");

  return (
    <div className={cn("mx-auto flex w-full flex-col gap-5 px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <section className="space-y-2.5">
        <div className="flex items-center justify-between gap-3 px-1">
          <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
            {t("settings.system.section_title")}
          </h2>
          {system_version_feedback ? (
            <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
              {system_version_feedback.message}
            </span>
          ) : null}
        </div>
        <div className={SETTINGS_CARD_CLASS_NAME}>
          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <PackageOpen className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.system.version_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {system_version_description}
                </p>
              </div>
            </div>
            <a
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)`}
              href={release_page_url}
              rel="noreferrer"
              target="_blank"
            >
              <ExternalLink className="h-3 w-3" />
              {t("settings.system.download_release")}
            </a>
          </div>
        </div>
      </section>

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
                <MonitorCog className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.general.default_model_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.default_model_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.general.default_model_label")}
              </span>
              <UiSelectMenu
                aria_label={t("settings.general.default_model_title")}
                button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
                class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
                disabled={provider_options_loading || !!default_model_saving_role || default_model_options.length === 0}
                leading={default_model_saving_role === "agent_runtime" ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
                menu_class_name="min-w-[260px]"
                on_change={(value) => handle_default_model_change(value, "agent_runtime")}
                options={default_model_options}
                placeholder={provider_options_loading
                  ? t("settings.general.default_model_loading")
                  : t("settings.general.default_model_empty")}
                size="xs"
                value={default_model_value}
              />
            </div>
          </div>

          <div className="border-t border-(--divider-subtle-color)" />

          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <Image className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.general.default_image_model_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.default_image_model_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.general.default_model_label")}
              </span>
              <UiSelectMenu
                aria_label={t("settings.general.default_image_model_title")}
                button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
                class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
                disabled={provider_options_loading || !!default_model_saving_role || default_image_model_options.length === 0}
                leading={default_model_saving_role === "image_generation" ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
                menu_class_name="min-w-[260px]"
                on_change={(value) => handle_default_model_change(value, "image_generation")}
                options={default_image_model_options}
                placeholder={provider_options_loading
                  ? t("settings.general.default_model_loading")
                  : t("settings.general.default_image_model_empty")}
                size="xs"
                value={default_image_model_value}
              />
            </div>
          </div>

          <div className="border-t border-(--divider-subtle-color)" />

          <div className={SETTINGS_ROW_CLASS_NAME}>
            <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
              <div className={SETTINGS_ICON_CLASS_NAME}>
                <Sparkles className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.general.default_background_model_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.default_background_model_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.general.default_model_label")}
              </span>
              <UiSelectMenu
                aria_label={t("settings.general.default_background_model_title")}
                button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
                class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
                disabled={provider_options_loading || !!default_model_saving_role || default_background_model_options.length === 0}
                leading={default_model_saving_role === "background_task" ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
                menu_class_name="min-w-[260px]"
                on_change={(value) => handle_default_model_change(value, "background_task")}
                options={default_background_model_options}
                placeholder={provider_options_loading
                  ? t("settings.general.default_model_loading")
                  : t("settings.general.default_background_model_empty")}
                size="xs"
                value={default_background_model_value}
              />
              {default_model_feedback ? (
                <span className="truncate text-[11px] text-(--text-soft)">
                  {default_model_feedback.message}
                </span>
              ) : null}
            </div>
          </div>

          <div className="border-t border-(--divider-subtle-color)" />

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
                <Terminal className="h-3.5 w-3.5" />
              </div>
              <div className="min-w-0">
                <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                  {t("settings.general.agent_runtime_title")}
                </h3>
                <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                  {t("settings.general.agent_runtime_description")}
                </p>
              </div>
            </div>
            <div className="flex min-w-0 flex-col gap-1.5">
              <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>
                {t("settings.general.agent_runtime_label")}
              </span>
              <SettingsSegmentedControl
                aria_label={t("settings.general.agent_runtime_label")}
                disabled={preferences_loading || preferences_saving || nxs_runtime_checking || nxs_runtime_downloading}
                on_change={handle_agent_runtime_kind_change}
                options={AGENT_RUNTIME_KIND_OPTIONS.map((option) => ({
                  value: option.value,
                  label: t(option.label_key),
                }))}
                value={agent_runtime_kind}
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
              className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)`}
              onClick={reset_all_tours}
              type="button"
            >
              <RotateCcw className="h-3 w-3" />
              {t("settings.onboarding_action_reset")}
            </button>
          </div>
        </div>
      </section>

      {desktop_available ? (
        <section className="space-y-2.5">
          <div className="flex items-center justify-between gap-3 px-1">
            <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
              {t("settings.desktop.section_title")}
            </h2>
            {desktop_feedback ? (
              <span className="min-w-0 truncate text-[11px] text-(--text-soft)">
                {desktop_feedback.message}
              </span>
            ) : null}
          </div>
          <div className={SETTINGS_CARD_CLASS_NAME}>
            <div className={SETTINGS_ROW_CLASS_NAME}>
              <div className={SETTINGS_TEXT_ROW_CLASS_NAME}>
                <div className={SETTINGS_ICON_CLASS_NAME}>
                  <MonitorCog className="h-3.5 w-3.5" />
                </div>
                <div className="min-w-0">
                  <h3 className={SETTINGS_ITEM_TITLE_CLASS_NAME}>
                    {t("settings.desktop.version_title")}
                  </h3>
                  <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
                    {desktop_version
                      ? t("settings.desktop.version_value")
                        .replace("{version}", desktop_version.app_version)
                        .replace("{build}", desktop_version.build_number)
                      : t("settings.desktop.version_loading")}
                  </p>
                </div>
              </div>
              <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
                <button
                  className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex min-w-0 items-center justify-center gap-1.5 rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:opacity-(--disabled-opacity)`}
                  disabled={exporting_logs}
                  onClick={handle_export_logs}
                  type="button"
                >
                  {exporting_logs ? <Loader2 className="h-3 w-3 animate-spin" /> : <Download className="h-3 w-3" />}
                  {t("settings.desktop.export_logs")}
                </button>
              </div>
            </div>
          </div>
        </section>
      ) : null}

      <section className="space-y-2.5">
        <div className="flex items-center justify-between gap-3 px-1">
          <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME}>
            {t("settings.general.section_permissions")}
          </h2>
          {preference_feedback ? (
            <span className="inline-flex items-center gap-1.5 text-[11px] text-(--destructive)">
              {preference_feedback.message}
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
              <UiSelectMenu
                aria_label={t("settings.general.default_permission_mode")}
                button_class_name={SETTINGS_SELECT_BUTTON_CLASS_NAME}
                class_name={SETTINGS_CONTROL_HEIGHT_CLASS_NAME}
                disabled={preferences_loading}
                id="default-permission-mode"
                menu_class_name="rounded-[12px]"
                on_change={handle_permission_mode_change}
                options={AGENT_PERMISSION_MODES.map((mode) => ({
                  value: mode.value,
                  label: t(mode.label_key),
                }))}
                placement="top"
                size="xs"
                value={permission_mode}
              />
              <p className="text-[11px] leading-4 text-(--text-soft)">
                {t(selected_permission_mode.description_key)}
              </p>
            </div>
          </div>
        </div>
      </section>
      {nxs_download_prompt_status ? (
        <UiDialogPortal>
          <UiDialogBackdrop
            described_by="nxs-runtime-download-message"
            labelled_by="nxs-runtime-download-title"
            on_close={() => {
              if (!nxs_runtime_downloading) {
                set_nxs_download_prompt_status(null);
              }
            }}
          >
            <UiDialogShell size="sm">
              <UiDialogHeader
                icon={<Download className="h-4 w-4" />}
                title={t("settings.general.agent_runtime_download_title")}
                title_id="nxs-runtime-download-title"
              />
              <UiDialogBody class_name="space-y-3">
                <p id="nxs-runtime-download-message" className="text-[13px] leading-5 text-(--text-default)">
                  {nxs_download_prompt_status.message || t("settings.general.agent_runtime_download_message")}
                </p>
                {nxs_download_prompt_status.path ? (
                  <p className="break-all rounded-[8px] border border-(--divider-subtle-color) px-2.5 py-2 text-[11px] leading-4 text-(--text-soft)">
                    {nxs_download_prompt_status.path}
                  </p>
                ) : null}
              </UiDialogBody>
              <UiDialogFooter>
                <button
                  className="inline-flex h-8 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) px-3 text-[12px] font-semibold text-(--text-default) hover:bg-(--surface-interactive-hover-background)"
                  disabled={nxs_runtime_downloading}
                  onClick={() => set_nxs_download_prompt_status(null)}
                  type="button"
                >
                  {t("common.cancel")}
                </button>
                <button
                  className="inline-flex h-8 items-center justify-center gap-1.5 rounded-[10px] bg-primary px-3 text-[12px] font-semibold text-white hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-70"
                  disabled={nxs_runtime_downloading}
                  onClick={handle_confirm_nxs_download}
                  type="button"
                >
                  {nxs_runtime_downloading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                  {nxs_runtime_downloading
                    ? t("settings.general.agent_runtime_downloading")
                    : t("settings.general.agent_runtime_download_confirm")}
                </button>
              </UiDialogFooter>
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}
    </div>
  );
}

export function SettingsPanel() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [active_tab, set_active_tab] = useState<SettingsTabKey>("general");
  const active_tab_config = SETTINGS_TABS.find((item) => item.key === active_tab) ?? SETTINGS_TABS[0];
  const ActiveIcon = active_tab_config.icon;
  const handle_back_to_workspace = useCallback(() => {
    if (is_desktop_bridge_available()) {
      void open_desktop_route(APP_ROUTE_PATHS.home).catch((error) => {
        console.error("[SettingsPanel] 桌面返回工作台失败:", error);
        navigate(APP_ROUTE_PATHS.home);
      });
      return;
    }
    navigate(APP_ROUTE_PATHS.home);
  }, [navigate]);

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
          trailing={(
            <WorkspaceSurfaceToolbarAction onClick={handle_back_to_workspace}>
              <ArrowLeft className="h-3.5 w-3.5" />
              {t("settings.back_to_workspace")}
            </WorkspaceSurfaceToolbarAction>
          )}
        />
      )}
    >
      {active_tab === "general" ? <GeneralSettingsSection /> : null}
      {active_tab === "personal" ? <PersonalSettingsPanel /> : null}
      {active_tab === "providers" ? <ProviderSettingsPanel embedded /> : null}
    </WorkspaceSurfaceScaffold>
  );
}
