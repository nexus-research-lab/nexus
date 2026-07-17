import { useCallback } from "react";

import { DEFAULT_AGENT_PERMISSION_MODE } from "@/lib/agent-options";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import { normalizeAgentRuntimeKind } from "@/types/settings/preferences";

import { useDefaultModelPreferences } from "./use-default-model-preferences";
import { useUserPreferences } from "./use-user-preferences";

export function useGeneralSettingsController() {
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
