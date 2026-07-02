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
  const { reset_all_tours: resetAllTours } = useOnboardingTour();
  const [preferences, setPreferences] = useState<UserPreferences>(() =>
    normalize_preferences(null),
  );
  const [preferencesLoading, setPreferencesLoading] = useState(true);
  const [preferencesSaving, setPreferencesSaving] = useState(false);
  const [preferenceFeedback, setPreferenceFeedback] =
    useState<PreferenceFeedback | null>(null);
  const [providerOptions, setProviderOptions] = useState<ProviderOption[]>(
    [],
  );
  const [backgroundProviderOptions, setBackgroundProviderOptions] =
    useState<ProviderOption[]>([]);
  const [imageProviderOptions, setImageProviderOptions] = useState<
    ProviderOption[]
  >([]);
  const [defaultModelValue, setDefaultModelValue] = useState("");
  const [defaultImageModelValue, setDefaultImageModelValue] =
    useState("");
  const [defaultBackgroundModelValue, setDefaultBackgroundModelValue] =
    useState("");
  const [providerOptionsLoading, setProviderOptionsLoading] =
    useState(true);
  const [defaultModelSavingRole, setDefaultModelSavingRole] =
    useState<DefaultModelPreferenceRole | null>(null);
  const [defaultModelFeedback, setDefaultModelFeedback] =
    useState<PreferenceFeedback | null>(null);
  const [nxsRuntimeChecking, setNxsRuntimeChecking] = useState(false);
  const preferencesRef = useRef(preferences);
  const lastSavedPreferencesRef = useRef<UserPreferences | null>(null);
  const providerDefaultSelectionRef = useRef({ provider: "", model: "" });
  const imageDefaultSelectionRef = useRef({ provider: "", model: "" });
  const saveSequenceRef = useRef(0);
  const agentRuntimeKind = normalize_agent_runtime_kind(
    preferences.agent_runtime_kind,
  );
  const permissionMode =
    preferences.default_agent_options.permission_mode ??
    DEFAULT_AGENT_PERMISSION_MODE;
  const syncDefaultModelValues = useCallback(
    (nextPreferences: UserPreferences) => {
      const agentProvider =
        nextPreferences.default_agent_options.provider?.trim() ||
        providerDefaultSelectionRef.current.provider;
      const agentModel =
        nextPreferences.default_agent_options.model?.trim() ||
        providerDefaultSelectionRef.current.model;
      set_default_agent_provider(agentProvider);
      set_default_agent_model(agentModel);
      setDefaultModelValue(
        encode_optional_model_selection(agentProvider, agentModel),
      );
      setDefaultImageModelValue(
        encode_optional_model_selection(
          nextPreferences.default_image_model_selection?.provider ||
            imageDefaultSelectionRef.current.provider,
          nextPreferences.default_image_model_selection?.model ||
            imageDefaultSelectionRef.current.model,
        ),
      );
      setDefaultBackgroundModelValue(
        encode_optional_model_selection(
          nextPreferences.default_background_model_selection?.provider,
          nextPreferences.default_background_model_selection?.model,
        ),
      );
    },
    [],
  );

  const loadProviderOptions = useCallback(
    async (runtimeKind?: AgentRuntimeKind) => {
      try {
        setProviderOptionsLoading(true);
        const selectedRuntimeKind =
          runtimeKind ??
          normalize_agent_runtime_kind(
            preferencesRef.current.agent_runtime_kind,
          );
        const result = await list_provider_options_api(selectedRuntimeKind);
        setProviderOptions(result.items ?? []);
        setBackgroundProviderOptions(
          result.background_items ?? result.items ?? [],
        );
        setImageProviderOptions(result.image_items ?? []);
        providerDefaultSelectionRef.current = {
          provider: result.default_provider?.trim() || "",
          model: result.default_model?.trim() || "",
        };
        imageDefaultSelectionRef.current = {
          provider: result.default_image_provider?.trim() || "",
          model: result.default_image_model?.trim() || "",
        };
        syncDefaultModelValues(preferencesRef.current);
        setDefaultModelFeedback(null);
      } catch (error) {
        setDefaultModelFeedback({
          message:
            error instanceof Error ? error.message : "默认对话模型加载失败",
        });
      } finally {
        setProviderOptionsLoading(false);
      }
    },
    [syncDefaultModelValues],
  );

  useEffect(() => {
    void loadProviderOptions(agentRuntimeKind);
  }, [agentRuntimeKind, loadProviderOptions]);

  useEffect(() => {
    let cancelled = false;
    const loadPreferences = async () => {
      try {
        setPreferencesLoading(true);
        const result = await get_user_preferences_api();
        if (cancelled) {
          return;
        }
        const normalized = normalize_preferences(result);
        set_user_preferences(normalized);
        setPreferences(normalized);
        preferencesRef.current = normalized;
        lastSavedPreferencesRef.current = normalized;
        syncDefaultModelValues(normalized);
        setPreferenceFeedback(null);
      } catch (error) {
        if (!cancelled) {
          setPreferenceFeedback({
            message:
              error instanceof Error
                ? error.message
                : t("settings.general.preferences_load_failed"),
          });
        }
      } finally {
        if (!cancelled) {
          setPreferencesLoading(false);
        }
      }
    };
    void loadPreferences();
    return () => {
      cancelled = true;
    };
  }, [syncDefaultModelValues, t]);

  const persistPreferences = useCallback(
    async (nextPreferences: UserPreferences) => {
      const sequence = saveSequenceRef.current + 1;
      saveSequenceRef.current = sequence;
      const normalized = normalize_preferences(nextPreferences);

      preferencesRef.current = normalized;
      setPreferences(normalized);
      set_user_preferences(normalized);
      setPreferenceFeedback(null);
      setPreferencesSaving(true);

      try {
        const result = await update_user_preferences_api(
          build_preferences_update_payload(normalized),
        );
        if (saveSequenceRef.current !== sequence) {
          return;
        }
        const saved = normalize_preferences(result);
        preferencesRef.current = saved;
        lastSavedPreferencesRef.current = saved;
        set_user_preferences(saved);
        setPreferences(saved);
      } catch (error) {
        if (saveSequenceRef.current !== sequence) {
          return;
        }
        const fallback = lastSavedPreferencesRef.current;
        if (fallback) {
          preferencesRef.current = fallback;
          set_user_preferences(fallback);
          setPreferences(fallback);
        }
        setPreferenceFeedback({
          message:
            error instanceof Error
              ? error.message
              : t("settings.general.preferences_save_failed"),
        });
      } finally {
        if (saveSequenceRef.current === sequence) {
          setPreferencesSaving(false);
        }
      }
    },
    [t],
  );

  const handleDeliveryPolicyChange = useCallback(
    (value: AgentConversationDefaultDeliveryPolicy) => {
      const currentPreferences = preferencesRef.current;
      void persistPreferences({
        ...currentPreferences,
        chat_default_delivery_policy: value,
      });
    },
    [persistPreferences],
  );

  const handleAgentSdkDiagnosticsChange = useCallback(
    (checked: boolean) => {
      const currentPreferences = preferencesRef.current;
      void persistPreferences({
        ...currentPreferences,
        agent_sdk_diagnostics_enabled: checked,
      });
    },
    [persistPreferences],
  );

  const handleAgentRuntimeKindChange = useCallback(
    (value: AgentRuntimeKind) => {
      const currentPreferences = preferencesRef.current;
      if (
        value ===
        normalize_agent_runtime_kind(currentPreferences.agent_runtime_kind)
      ) {
        return;
      }
      if (value !== "nxs") {
        void (async () => {
          await persistPreferences({
            ...currentPreferences,
            agent_runtime_kind: value,
          });
          await loadProviderOptions(value);
        })();
        return;
      }
      void (async () => {
        setNxsRuntimeChecking(true);
        setPreferenceFeedback(null);
        try {
          const status = await get_nxs_runtime_status_api();
          if (status.available) {
            await persistPreferences({
              ...preferencesRef.current,
              agent_runtime_kind: "nxs",
            });
            await loadProviderOptions("nxs");
            return;
          }
          setPreferenceFeedback({
            message:
              status.message ||
              t("settings.general.agent_runtime_nxs_unavailable"),
          });
        } catch (error) {
          setPreferenceFeedback({
            message:
              error instanceof Error
                ? error.message
                : t("settings.general.agent_runtime_check_failed"),
          });
        } finally {
          setNxsRuntimeChecking(false);
        }
      })();
    },
    [loadProviderOptions, persistPreferences, t],
  );

  const handlePermissionModeChange = useCallback(
    (value: string) => {
      const currentPreferences = preferencesRef.current;
      void persistPreferences({
        ...currentPreferences,
        default_agent_options: {
          ...currentPreferences.default_agent_options,
          permission_mode: value,
        },
      });
    },
    [persistPreferences],
  );

  const defaultModelOptions = useMemo(
    () => build_default_model_options(providerOptions),
    [providerOptions],
  );
  const defaultImageModelOptions = useMemo(
    () => build_default_model_options(imageProviderOptions),
    [imageProviderOptions],
  );
  const defaultBackgroundModelOptions = useMemo(
    () => build_default_model_options(backgroundProviderOptions),
    [backgroundProviderOptions],
  );

  const handleDefaultModelChange = useCallback(
    (value: string, role: DefaultModelPreferenceRole) => {
      const selection = decode_default_model_value(value);
      if (!selection || defaultModelSavingRole) {
        return;
      }
      void (async () => {
        setDefaultModelSavingRole(role);
        setDefaultModelFeedback(null);
        const previousValue =
          role === "image_generation"
            ? defaultImageModelValue
            : role === "background_task"
              ? defaultBackgroundModelValue
              : defaultModelValue;
        if (role === "image_generation") {
          setDefaultImageModelValue(value);
        } else if (role === "background_task") {
          setDefaultBackgroundModelValue(value);
        } else {
          setDefaultModelValue(value);
        }
        try {
          const currentPreferences = preferencesRef.current;
          const nextPreferences = normalize_preferences({
            ...currentPreferences,
            default_agent_options:
              role === "agent_runtime"
                ? {
                  ...currentPreferences.default_agent_options,
                  provider: selection.provider,
                  model: selection.model,
                }
                : currentPreferences.default_agent_options,
            default_image_model_selection:
              role === "image_generation"
                ? { provider: selection.provider, model: selection.model }
                : currentPreferences.default_image_model_selection,
            default_background_model_selection:
              role === "background_task"
                ? { provider: selection.provider, model: selection.model }
                : currentPreferences.default_background_model_selection,
          });
          preferencesRef.current = nextPreferences;
          setPreferences(nextPreferences);
          set_user_preferences(nextPreferences);
          const result = await update_user_preferences_api(
            build_preferences_update_payload(nextPreferences),
          );
          const saved = normalize_preferences(result);
          preferencesRef.current = saved;
          lastSavedPreferencesRef.current = saved;
          setPreferences(saved);
          set_user_preferences(saved);
          if (role === "agent_runtime") {
            set_default_agent_provider(selection.provider);
            set_default_agent_model(selection.model);
          }
        } catch (error) {
          const fallback = lastSavedPreferencesRef.current;
          if (fallback) {
            preferencesRef.current = fallback;
            setPreferences(fallback);
            set_user_preferences(fallback);
            if (role === "agent_runtime") {
              set_default_agent_provider(
                fallback.default_agent_options.provider,
              );
              set_default_agent_model(fallback.default_agent_options.model);
            }
          }
          if (role === "image_generation") {
            setDefaultImageModelValue(previousValue);
          } else if (role === "background_task") {
            setDefaultBackgroundModelValue(previousValue);
          } else {
            setDefaultModelValue(previousValue);
          }
          setDefaultModelFeedback({
            message:
              error instanceof Error ? error.message : "默认对话模型保存失败",
          });
        } finally {
          setDefaultModelSavingRole(null);
        }
      })();
    },
    [
      defaultBackgroundModelValue,
      defaultImageModelValue,
      defaultModelSavingRole,
      defaultModelValue,
    ],
  );

  return (
    <div className={cn("mx-auto flex w-full flex-col gap-5 px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
      <SettingsSystemSection />

      <SettingsAppearanceSection />

      <SettingsGeneralBehaviorSection
        agent_runtime_kind={agentRuntimeKind}
        agent_sdk_diagnostics_enabled={
          preferences.agent_sdk_diagnostics_enabled === true
        }
        chat_default_delivery_policy={preferences.chat_default_delivery_policy}
        default_background_model_options={defaultBackgroundModelOptions}
        default_background_model_value={defaultBackgroundModelValue}
        default_image_model_options={defaultImageModelOptions}
        default_image_model_value={defaultImageModelValue}
        default_model_feedback_message={defaultModelFeedback?.message}
        default_model_options={defaultModelOptions}
        default_model_saving_role={defaultModelSavingRole}
        default_model_value={defaultModelValue}
        nxs_runtime_checking={nxsRuntimeChecking}
        on_agent_runtime_kind_change={handleAgentRuntimeKindChange}
        on_agent_sdk_diagnostics_change={handleAgentSdkDiagnosticsChange}
        on_default_delivery_policy_change={handleDeliveryPolicyChange}
        on_default_model_change={handleDefaultModelChange}
        on_reset_tours={resetAllTours}
        preferences_loading={preferencesLoading}
        preferences_saving={preferencesSaving}
        provider_options_loading={providerOptionsLoading}
      />

      <SettingsDesktopSection />

      <SettingsPermissionsSection
        feedback_message={preferenceFeedback?.message}
        on_permission_mode_change={handlePermissionModeChange}
        permission_mode={permissionMode}
        preferences_loading={preferencesLoading}
      />
    </div>
  );
}
