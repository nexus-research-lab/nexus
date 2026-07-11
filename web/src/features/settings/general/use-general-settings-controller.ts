import { useCallback, useState } from "react";

import { DEFAULT_AGENT_PERMISSION_MODE } from "@/features/agents/options/agent-options-constants";
import { getNxsRuntimeStatusApi } from "@/lib/api/settings/runtime-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useOnboardingTour } from "@/shared/ui/onboarding/use-onboarding-tour";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import {
  normalizeAgentRuntimeKind,
  type AgentRuntimeKind,
} from "@/types/settings/preferences";

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
    setFeedback,
    updatePreferences,
  } = preferencesStore;
  const [nxsRuntimeChecking, setNxsRuntimeChecking] = useState(false);
  const preferencesBusy = saving || nxsRuntimeChecking;
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
  const selectRuntime = useCallback((value: AgentRuntimeKind) => {
    updatePreferences((current) => ({
      ...current,
      agent_runtime_kind: value,
    }));
  }, [updatePreferences]);
  const verifyAndSelectNxs = useCallback(async () => {
    setNxsRuntimeChecking(true);
    setFeedback(null);
    try {
      const status = await getNxsRuntimeStatusApi();
      if (status.available) {
        selectRuntime("nxs");
        return;
      }
      setFeedback({
        message: status.message
          || t("settings.general.agent_runtime_nxs_unavailable"),
      });
    } catch (error) {
      setFeedback({
        message: error instanceof Error
          ? error.message
          : t("settings.general.agent_runtime_check_failed"),
      });
    } finally {
      setNxsRuntimeChecking(false);
    }
  }, [selectRuntime, setFeedback, t]);
  const handleAgentRuntimeKindChange = useCallback((value: AgentRuntimeKind) => {
    if (value === agentRuntimeKind) {
      return;
    }
    if (value === "nxs") {
      void verifyAndSelectNxs();
      return;
    }
    selectRuntime(value);
  }, [agentRuntimeKind, selectRuntime, verifyAndSelectNxs]);
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
      agentRuntimeKind,
      agentSdkDiagnosticsEnabled:
        preferences.agent_sdk_diagnostics_enabled === true,
      chatDefaultDeliveryPolicy: preferences.chat_default_delivery_policy,
      defaultBackgroundModelOptions: defaultModels.options.background,
      defaultBackgroundModelValue: defaultModels.values.background,
      defaultImageModelOptions: defaultModels.options.image,
      defaultImageModelValue: defaultModels.values.image,
      defaultModelFeedbackMessage: defaultModels.feedbackMessage,
      defaultModelOptions: defaultModels.options.agent,
      defaultModelSavingRole: defaultModels.savingRole,
      defaultModelValue: defaultModels.values.agent,
      nxsRuntimeChecking,
      onAgentRuntimeKindChange: handleAgentRuntimeKindChange,
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
