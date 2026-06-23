"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  set_default_agent_model,
  set_default_agent_provider,
  set_user_preferences,
} from "@/config/options";
import {
  DEFAULT_AGENT_PERMISSION_MODE,
} from "@/features/agents/options/agent-options-constants";
import {
  list_provider_options_api,
} from "@/lib/api/provider-config-api";
import { get_nxs_runtime_status_api } from "@/lib/api/runtime-api";
import {
  get_user_preferences_api,
  update_user_preferences_api,
} from "@/lib/api/settings-preferences-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { ProviderOption } from "@/types/capability/provider";
import {
  normalize_agent_runtime_kind,
  type AgentRuntimeKind,
  type UserPreferences,
} from "@/types/settings/preferences";

import { SettingsAppearanceSection } from "./settings-appearance-section";
import { SettingsDesktopSection } from "./settings-desktop-section";
import { SettingsGeneralBehaviorSection } from "./settings-general-behavior-section";
import { SettingsPermissionsSection } from "./settings-permissions-section";
import {
  type DefaultModelPreferenceRole,
  type PreferenceFeedback,
  build_default_model_options,
  build_preferences_update_payload,
  decode_default_model_value,
  encode_optional_model_selection,
  normalize_preferences,
} from "./settings-preferences-model";
import { SettingsSystemSection } from "./settings-system-section";

export function SettingsGeneralSection() {
  const { t } = useI18n();
  const { reset_all_tours } = useOnboardingTour();
  const [preferences, set_preferences] = useState<UserPreferences>(() =>
    normalize_preferences(null),
  );
  const [preferences_loading, set_preferences_loading] = useState(true);
  const [preferences_saving, set_preferences_saving] = useState(false);
  const [preference_feedback, set_preference_feedback] =
    useState<PreferenceFeedback | null>(null);
  const [provider_options, set_provider_options] = useState<ProviderOption[]>(
    [],
  );
  const [background_provider_options, set_background_provider_options] =
    useState<ProviderOption[]>([]);
  const [image_provider_options, set_image_provider_options] = useState<
    ProviderOption[]
  >([]);
  const [default_model_value, set_default_model_value] = useState("");
  const [default_image_model_value, set_default_image_model_value] =
    useState("");
  const [default_background_model_value, set_default_background_model_value] =
    useState("");
  const [provider_options_loading, set_provider_options_loading] =
    useState(true);
  const [default_model_saving_role, set_default_model_saving_role] =
    useState<DefaultModelPreferenceRole | null>(null);
  const [default_model_feedback, set_default_model_feedback] =
    useState<PreferenceFeedback | null>(null);
  const [nxs_runtime_checking, set_nxs_runtime_checking] = useState(false);
  const preferences_ref = useRef(preferences);
  const last_saved_preferences_ref = useRef<UserPreferences | null>(null);
  const provider_default_selection_ref = useRef({ provider: "", model: "" });
  const image_default_selection_ref = useRef({ provider: "", model: "" });
  const save_sequence_ref = useRef(0);
  const agent_runtime_kind = normalize_agent_runtime_kind(
    preferences.agent_runtime_kind,
  );
  const permission_mode =
    preferences.default_agent_options.permission_mode ??
    DEFAULT_AGENT_PERMISSION_MODE;
  const sync_default_model_values = useCallback(
    (next_preferences: UserPreferences) => {
      const agent_provider =
        next_preferences.default_agent_options.provider?.trim() ||
        provider_default_selection_ref.current.provider;
      const agent_model =
        next_preferences.default_agent_options.model?.trim() ||
        provider_default_selection_ref.current.model;
      set_default_agent_provider(agent_provider);
      set_default_agent_model(agent_model);
      set_default_model_value(
        encode_optional_model_selection(agent_provider, agent_model),
      );
      set_default_image_model_value(
        encode_optional_model_selection(
          next_preferences.default_image_model_selection?.provider ||
            image_default_selection_ref.current.provider,
          next_preferences.default_image_model_selection?.model ||
            image_default_selection_ref.current.model,
        ),
      );
      set_default_background_model_value(
        encode_optional_model_selection(
          next_preferences.default_background_model_selection?.provider,
          next_preferences.default_background_model_selection?.model,
        ),
      );
    },
    [],
  );

  const load_provider_options = useCallback(
    async (runtime_kind?: AgentRuntimeKind) => {
      try {
        set_provider_options_loading(true);
        const selected_runtime_kind =
          runtime_kind ??
          normalize_agent_runtime_kind(
            preferences_ref.current.agent_runtime_kind,
          );
        const result = await list_provider_options_api(selected_runtime_kind);
        set_provider_options(result.items ?? []);
        set_background_provider_options(
          result.background_items ?? result.items ?? [],
        );
        set_image_provider_options(result.image_items ?? []);
        provider_default_selection_ref.current = {
          provider: result.default_provider?.trim() || "",
          model: result.default_model?.trim() || "",
        };
        image_default_selection_ref.current = {
          provider: result.default_image_provider?.trim() || "",
          model: result.default_image_model?.trim() || "",
        };
        sync_default_model_values(preferences_ref.current);
        set_default_model_feedback(null);
      } catch (error) {
        set_default_model_feedback({
          message:
            error instanceof Error ? error.message : "默认对话模型加载失败",
        });
      } finally {
        set_provider_options_loading(false);
      }
    },
    [sync_default_model_values],
  );

  useEffect(() => {
    void load_provider_options(agent_runtime_kind);
  }, [agent_runtime_kind, load_provider_options]);

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
        sync_default_model_values(normalized);
        set_preference_feedback(null);
      } catch (error) {
        if (!cancelled) {
          set_preference_feedback({
            message:
              error instanceof Error
                ? error.message
                : t("settings.general.preferences_load_failed"),
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
  }, [sync_default_model_values, t]);

  const persist_preferences = useCallback(
    async (next_preferences: UserPreferences) => {
      const sequence = save_sequence_ref.current + 1;
      save_sequence_ref.current = sequence;
      const normalized = normalize_preferences(next_preferences);

      preferences_ref.current = normalized;
      set_preferences(normalized);
      set_user_preferences(normalized);
      set_preference_feedback(null);
      set_preferences_saving(true);

      try {
        const result = await update_user_preferences_api(
          build_preferences_update_payload(normalized),
        );
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
          message:
            error instanceof Error
              ? error.message
              : t("settings.general.preferences_save_failed"),
        });
      } finally {
        if (save_sequence_ref.current === sequence) {
          set_preferences_saving(false);
        }
      }
    },
    [t],
  );

  const handle_delivery_policy_change = useCallback(
    (value: AgentConversationDefaultDeliveryPolicy) => {
      const current_preferences = preferences_ref.current;
      void persist_preferences({
        ...current_preferences,
        chat_default_delivery_policy: value,
      });
    },
    [persist_preferences],
  );

  const handle_agent_sdk_diagnostics_change = useCallback(
    (checked: boolean) => {
      const current_preferences = preferences_ref.current;
      void persist_preferences({
        ...current_preferences,
        agent_sdk_diagnostics_enabled: checked,
      });
    },
    [persist_preferences],
  );

  const handle_agent_runtime_kind_change = useCallback(
    (value: AgentRuntimeKind) => {
      const current_preferences = preferences_ref.current;
      if (
        value ===
        normalize_agent_runtime_kind(current_preferences.agent_runtime_kind)
      ) {
        return;
      }
      if (value !== "nxs") {
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
          set_preference_feedback({
            message:
              status.message ||
              t("settings.general.agent_runtime_nxs_unavailable"),
          });
        } catch (error) {
          set_preference_feedback({
            message:
              error instanceof Error
                ? error.message
                : t("settings.general.agent_runtime_check_failed"),
          });
        } finally {
          set_nxs_runtime_checking(false);
        }
      })();
    },
    [load_provider_options, persist_preferences, t],
  );

  const handle_permission_mode_change = useCallback(
    (value: string) => {
      const current_preferences = preferences_ref.current;
      void persist_preferences({
        ...current_preferences,
        default_agent_options: {
          ...current_preferences.default_agent_options,
          permission_mode: value,
        },
      });
    },
    [persist_preferences],
  );

  const default_model_options = useMemo(
    () => build_default_model_options(provider_options),
    [provider_options],
  );
  const default_image_model_options = useMemo(
    () => build_default_model_options(image_provider_options),
    [image_provider_options],
  );
  const default_background_model_options = useMemo(
    () => build_default_model_options(background_provider_options),
    [background_provider_options],
  );

  const handle_default_model_change = useCallback(
    (value: string, role: DefaultModelPreferenceRole) => {
      const selection = decode_default_model_value(value);
      if (!selection || default_model_saving_role) {
        return;
      }
      void (async () => {
        set_default_model_saving_role(role);
        set_default_model_feedback(null);
        const previous_value =
          role === "image_generation"
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
            default_agent_options:
              role === "agent_runtime"
                ? {
                  ...current_preferences.default_agent_options,
                  provider: selection.provider,
                  model: selection.model,
                }
                : current_preferences.default_agent_options,
            default_image_model_selection:
              role === "image_generation"
                ? { provider: selection.provider, model: selection.model }
                : current_preferences.default_image_model_selection,
            default_background_model_selection:
              role === "background_task"
                ? { provider: selection.provider, model: selection.model }
                : current_preferences.default_background_model_selection,
          });
          preferences_ref.current = next_preferences;
          set_preferences(next_preferences);
          set_user_preferences(next_preferences);
          const result = await update_user_preferences_api(
            build_preferences_update_payload(next_preferences),
          );
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
              set_default_agent_provider(
                fallback.default_agent_options.provider,
              );
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
            message:
              error instanceof Error ? error.message : "默认对话模型保存失败",
          });
        } finally {
          set_default_model_saving_role(null);
        }
      })();
    },
    [
      default_background_model_value,
      default_image_model_value,
      default_model_saving_role,
      default_model_value,
    ],
  );

  return (
    <div className={cn("mx-auto flex w-full flex-col gap-5 px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <SettingsSystemSection />

      <SettingsAppearanceSection />

      <SettingsGeneralBehaviorSection
        agent_runtime_kind={agent_runtime_kind}
        agent_sdk_diagnostics_enabled={
          preferences.agent_sdk_diagnostics_enabled === true
        }
        chat_default_delivery_policy={preferences.chat_default_delivery_policy}
        default_background_model_options={default_background_model_options}
        default_background_model_value={default_background_model_value}
        default_image_model_options={default_image_model_options}
        default_image_model_value={default_image_model_value}
        default_model_feedback_message={default_model_feedback?.message}
        default_model_options={default_model_options}
        default_model_saving_role={default_model_saving_role}
        default_model_value={default_model_value}
        nxs_runtime_checking={nxs_runtime_checking}
        on_agent_runtime_kind_change={handle_agent_runtime_kind_change}
        on_agent_sdk_diagnostics_change={handle_agent_sdk_diagnostics_change}
        on_default_delivery_policy_change={handle_delivery_policy_change}
        on_default_model_change={handle_default_model_change}
        on_reset_tours={reset_all_tours}
        preferences_loading={preferences_loading}
        preferences_saving={preferences_saving}
        provider_options_loading={provider_options_loading}
      />

      <SettingsDesktopSection />

      <SettingsPermissionsSection
        feedback_message={preference_feedback?.message}
        on_permission_mode_change={handle_permission_mode_change}
        permission_mode={permission_mode}
        preferences_loading={preferences_loading}
      />
    </div>
  );
}
