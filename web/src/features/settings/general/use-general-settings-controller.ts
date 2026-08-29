import { useCallback, useEffect, useRef, useState } from "react";

import { DEFAULT_AGENT_PERMISSION_MODE } from "@/lib/agent-options";
import { getEchoApi, updateEchoApi } from "@/lib/api/settings/echo-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import { normalizeAgentRuntimeKind } from "@/types/settings/preferences";

import { useDefaultModelPreferences } from "./use-default-model-preferences";
import { useUserPreferences } from "./use-user-preferences";

export function useGeneralSettingsController() {
  const { t } = useI18n();
  const { resetAllTours } = useOnboardingTour();
  const preferencesStore = useUserPreferences();
  const {
    feedback,
    getCurrentPreferences,
    loading,
    persistPreferences,
    preferences,
    saving,
    updatePreferences,
  } = preferencesStore;
  const preferencesBusy = saving;
  const [echoEnabled, setEchoEnabled] = useState(false);
  const [echoLoading, setEchoLoading] = useState(true);
  const [echoSaving, setEchoSaving] = useState(false);
  const [echoFeedbackMessage, setEchoFeedbackMessage] = useState<string | null>(
    null,
  );
  const echoEnabledRef = useRef(false);
  const echoSavingRef = useRef(false);
  const agentRuntimeKind = normalizeAgentRuntimeKind(
    preferences.agent_runtime_kind,
  );
  const defaultModels = useDefaultModelPreferences({
    agentRuntimeKind,
    getCurrentPreferences,
    persistPreferences,
    preferences,
    preferencesSaving: preferencesBusy,
  });

  useEffect(() => {
    let cancelled = false;
    void getEchoApi()
      .then((settings) => {
        if (cancelled) {
          return;
        }
        echoEnabledRef.current = settings.enabled;
        setEchoEnabled(settings.enabled);
        setEchoFeedbackMessage(null);
      })
      .catch(() => {
        if (!cancelled) {
          setEchoFeedbackMessage(t("settings.general.echo_load_failed"));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setEchoLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [t]);

  const handleEchoEnabledChange = useCallback((enabled: boolean) => {
    if (echoSavingRef.current || enabled === echoEnabledRef.current) {
      return;
    }
    const previous = echoEnabledRef.current;
    echoSavingRef.current = true;
    echoEnabledRef.current = enabled;
    setEchoEnabled(enabled);
    setEchoSaving(true);
    setEchoFeedbackMessage(null);
    void updateEchoApi({ enabled })
      .then((settings) => {
        echoEnabledRef.current = settings.enabled;
        setEchoEnabled(settings.enabled);
      })
      .catch(() => {
        echoEnabledRef.current = previous;
        setEchoEnabled(previous);
        setEchoFeedbackMessage(t("settings.general.echo_save_failed"));
      })
      .finally(() => {
        echoSavingRef.current = false;
        setEchoSaving(false);
      });
  }, [t]);

  const handleDeliveryPolicyChange = useCallback(
    (value: AgentConversationDefaultDeliveryPolicy) => {
      updatePreferences((current) => ({
        ...current,
        chat_default_delivery_policy: value,
      }));
    },
    [updatePreferences],
  );
  const handleAgentSdkDiagnosticsChange = useCallback(
    (checked: boolean) => {
      updatePreferences((current) => ({
        ...current,
        agent_sdk_diagnostics_enabled: checked,
      }));
    },
    [updatePreferences],
  );
  const handleEmotionEnabledChange = useCallback(
    (checked: boolean) => {
      updatePreferences((current) => ({
        ...current,
        emotion_enabled: checked,
      }));
    },
    [updatePreferences],
  );
  const handleAutoMemoryEnabledChange = useCallback(
    (checked: boolean) => {
      updatePreferences((current) => ({
        ...current,
        runtime_settings: {
          ...current.runtime_settings,
          nxs: {
            ...current.runtime_settings?.nxs,
            auto_memory_enabled: checked,
          },
        },
      }));
    },
    [updatePreferences],
  );
  const handleAutoDreamEnabledChange = useCallback(
    (checked: boolean) => {
      updatePreferences((current) => ({
        ...current,
        runtime_settings: {
          ...current.runtime_settings,
          nxs: {
            ...current.runtime_settings?.nxs,
            auto_dream_enabled: checked,
          },
        },
      }));
    },
    [updatePreferences],
  );
  const handlePermissionModeChange = useCallback((value: string) => {
    updatePreferences((current) => ({
      ...current,
      default_agent_options: {
        ...current.default_agent_options,
        permission_mode: value,
      },
    }));
  }, [updatePreferences]);

  return {
    behavior: {
      agentSdkDiagnosticsEnabled:
        preferences.agent_sdk_diagnostics_enabled === true,
      chatDefaultDeliveryPolicy: preferences.chat_default_delivery_policy,
      emotionEnabled: preferences.emotion_enabled === true,
      autoMemoryEnabled:
        preferences.runtime_settings?.nxs?.auto_memory_enabled ?? true,
      autoDreamEnabled:
        preferences.runtime_settings?.nxs?.auto_dream_enabled ?? true,
      echoEnabled,
      echoFeedbackMessage,
      echoLoading,
      echoSaving,
      defaultBackgroundModelOptions: defaultModels.options.background,
      defaultBackgroundModelValue: defaultModels.values.background,
      defaultImageModelOptions: defaultModels.options.image,
      defaultImageModelValue: defaultModels.values.image,
      defaultVisionModelOptions: defaultModels.options.vision,
      defaultVisionModelValue: defaultModels.values.vision,
      defaultModelFeedbackMessage: defaultModels.feedbackMessage,
      defaultModelOptions: defaultModels.options.agent,
      defaultModelSavingRole: defaultModels.savingRole,
      defaultModelValue: defaultModels.values.agent,
      onAgentSdkDiagnosticsChange: handleAgentSdkDiagnosticsChange,
      onAutoMemoryEnabledChange: handleAutoMemoryEnabledChange,
      onAutoDreamEnabledChange: handleAutoDreamEnabledChange,
      onEmotionEnabledChange: handleEmotionEnabledChange,
      onEchoEnabledChange: handleEchoEnabledChange,
      onDefaultDeliveryPolicyChange: handleDeliveryPolicyChange,
      onDefaultModelChange: defaultModels.handleChange,
      onResetTours: resetAllTours,
      preferencesLoading: loading,
      preferencesSaving: preferencesBusy,
      providerOptionsLoading: defaultModels.loading,
    },
    permissions: {
      feedbackMessage: feedback?.message,
      onPermissionModeChange: handlePermissionModeChange,
      permissionMode:
        preferences.default_agent_options.permission_mode
        ?? DEFAULT_AGENT_PERMISSION_MODE,
      preferencesLoading: loading,
      preferencesSaving: preferencesBusy,
    },
  };
}
