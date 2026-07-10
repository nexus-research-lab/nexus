"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { setUserPreferences } from "@/config/options";
import {
  DEFAULT_AGENT_PERMISSION_MODE,
} from "@/features/agents/options/agent-options-constants";
import {
  listProviderOptionsApi,
} from "@/lib/api/provider-config-api";
import { getNxsRuntimeStatusApi } from "@/lib/api/runtime-api";
import {
  getUserPreferencesApi,
  updateUserPreferencesApi,
} from "@/lib/api/settings-preferences-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { ProviderOption } from "@/types/capability/provider";
import {
  normalizeAgentRuntimeKind,
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
  buildDefaultModelOptions,
  buildPreferencesUpdatePayload,
  decodeDefaultModelValue,
  encodeOptionalModelSelection,
  normalizePreferences,
} from "./settings-preferences-model";
import { SettingsSystemSection } from "./settings-system-section";
import { SettingsWorkspaceSection } from "./settings-workspace-section";

export function SettingsGeneralSection() {
  const { t } = useI18n();
  const { resetAllTours: resetAllTours } = useOnboardingTour();
  const [preferences, setPreferences] = useState<UserPreferences>(() =>
    normalizePreferences(null),
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
  const agentRuntimeKind = normalizeAgentRuntimeKind(
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
      setDefaultModelValue(
        encodeOptionalModelSelection(agentProvider, agentModel),
      );
      setDefaultImageModelValue(
        encodeOptionalModelSelection(
          nextPreferences.default_image_model_selection?.provider ||
            imageDefaultSelectionRef.current.provider,
          nextPreferences.default_image_model_selection?.model ||
            imageDefaultSelectionRef.current.model,
        ),
      );
      setDefaultBackgroundModelValue(
        encodeOptionalModelSelection(
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
          normalizeAgentRuntimeKind(
            preferencesRef.current.agent_runtime_kind,
          );
        const result = await listProviderOptionsApi(selectedRuntimeKind);
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
        const result = await getUserPreferencesApi();
        if (cancelled) {
          return;
        }
        const normalized = normalizePreferences(result);
        setUserPreferences(normalized);
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
      const normalized = normalizePreferences(nextPreferences);

      preferencesRef.current = normalized;
      setPreferences(normalized);
      setUserPreferences(normalized);
      setPreferenceFeedback(null);
      setPreferencesSaving(true);

      try {
        const result = await updateUserPreferencesApi(
          buildPreferencesUpdatePayload(normalized),
        );
        if (saveSequenceRef.current !== sequence) {
          return;
        }
        const saved = normalizePreferences(result);
        preferencesRef.current = saved;
        lastSavedPreferencesRef.current = saved;
        setUserPreferences(saved);
        setPreferences(saved);
      } catch (error) {
        if (saveSequenceRef.current !== sequence) {
          return;
        }
        const fallback = lastSavedPreferencesRef.current;
        if (fallback) {
          preferencesRef.current = fallback;
          setUserPreferences(fallback);
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
        normalizeAgentRuntimeKind(currentPreferences.agent_runtime_kind)
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
          const status = await getNxsRuntimeStatusApi();
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
    () => buildDefaultModelOptions(
      providerOptions,
      t("settings.providers.subscription_badge"),
    ),
    [providerOptions, t],
  );
  const defaultImageModelOptions = useMemo(
    () => buildDefaultModelOptions(
      imageProviderOptions,
      t("settings.providers.subscription_badge"),
    ),
    [imageProviderOptions, t],
  );
  const defaultBackgroundModelOptions = useMemo(
    () => buildDefaultModelOptions(
      backgroundProviderOptions,
      t("settings.providers.subscription_badge"),
    ),
    [backgroundProviderOptions, t],
  );

  const handleDefaultModelChange = useCallback(
    (value: string, role: DefaultModelPreferenceRole) => {
      const selection = decodeDefaultModelValue(value);
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
          const nextPreferences = normalizePreferences({
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
          setUserPreferences(nextPreferences);
          const result = await updateUserPreferencesApi(
            buildPreferencesUpdatePayload(nextPreferences),
          );
          const saved = normalizePreferences(result);
          preferencesRef.current = saved;
          lastSavedPreferencesRef.current = saved;
          setPreferences(saved);
          setUserPreferences(saved);
        } catch (error) {
          const fallback = lastSavedPreferencesRef.current;
          if (fallback) {
            preferencesRef.current = fallback;
            setPreferences(fallback);
            setUserPreferences(fallback);
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
        agentRuntimeKind={agentRuntimeKind}
        agentSdkDiagnosticsEnabled={
          preferences.agent_sdk_diagnostics_enabled === true
        }
        chatDefaultDeliveryPolicy={preferences.chat_default_delivery_policy}
        defaultBackgroundModelOptions={defaultBackgroundModelOptions}
        defaultBackgroundModelValue={defaultBackgroundModelValue}
        defaultImageModelOptions={defaultImageModelOptions}
        defaultImageModelValue={defaultImageModelValue}
        defaultModelFeedbackMessage={defaultModelFeedback?.message}
        defaultModelOptions={defaultModelOptions}
        defaultModelSavingRole={defaultModelSavingRole}
        defaultModelValue={defaultModelValue}
        nxsRuntimeChecking={nxsRuntimeChecking}
        onAgentRuntimeKindChange={handleAgentRuntimeKindChange}
        onAgentSdkDiagnosticsChange={handleAgentSdkDiagnosticsChange}
        onDefaultDeliveryPolicyChange={handleDeliveryPolicyChange}
        onDefaultModelChange={handleDefaultModelChange}
        onResetTours={resetAllTours}
        preferencesLoading={preferencesLoading}
        preferencesSaving={preferencesSaving}
        providerOptionsLoading={providerOptionsLoading}
      />

      <SettingsWorkspaceSection />

      <SettingsDesktopSection />

      <SettingsPermissionsSection
        feedbackMessage={preferenceFeedback?.message}
        onPermissionModeChange={handlePermissionModeChange}
        permissionMode={permissionMode}
        preferencesLoading={preferencesLoading}
      />
    </div>
  );
}
