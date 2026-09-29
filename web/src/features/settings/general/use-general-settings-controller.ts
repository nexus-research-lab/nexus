/**
 * INPUT: Preferences 和 Echo的分域控制器状态。
 * OUTPUT: General/Permissions 页面所需的稳定视图模型和恢复动作。
 * POS: 通用设置装配层；不复制各领域的读写与对账逻辑。
 */
import { useCallback } from "react";

import { DEFAULT_AGENT_PERMISSION_MODE } from "@/lib/agent-options";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";

import { useEchoSettings } from "./use-echo-settings";
import { useUserPreferences } from "./use-user-preferences";

export function useGeneralSettingsController() {
  const preferencesStore = useUserPreferences();
  const {
    acceptExternalAggregateSnapshot,
    feedback,
    hasUnresolvedMutation,
    loading,
    preferences,
    recovery,
    saving,
    updatePreferences,
    writable,
  } = preferencesStore;
  const handleEchoAggregateSnapshot = useCallback((
    expectedVersion: number,
    snapshot: { enabled: boolean; version: number },
  ) => {
    if (!acceptExternalAggregateSnapshot(
      expectedVersion,
      snapshot,
    )) {
      recovery.checkLatest();
    }
  }, [acceptExternalAggregateSnapshot, recovery]);
  const preferencesVersion = preferences.version;
  const echo = useEchoSettings({
    aggregate: typeof preferencesVersion === "number"
      && Number.isSafeInteger(preferencesVersion)
      && preferencesVersion > 0
      ? {
          enabled: preferences.echo_enabled === true,
          version: preferencesVersion,
        }
      : null,
    aggregateLoading: loading,
    blocked: loading
      || saving
      || !writable
      || recovery.checking
      || hasUnresolvedMutation,
    onAggregateSnapshot: handleEchoAggregateSnapshot,
  });
  const preferencesBusy = saving
    || !writable
    || echo.saving
    || echo.recovery.checking
    || echo.hasUnresolvedMutation;
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
      echoDisabled: echo.disabled,
      echoEnabled: echo.enabled,
      echoFeedback: echo.feedback,
      echoLoading: echo.loading,
      echoRecovery: echo.recovery,
      echoSaving: echo.saving,
      onAgentSdkDiagnosticsChange: handleAgentSdkDiagnosticsChange,
      onAutoMemoryEnabledChange: handleAutoMemoryEnabledChange,
      onAutoDreamEnabledChange: handleAutoDreamEnabledChange,
      onEmotionEnabledChange: handleEmotionEnabledChange,
      onEchoEnabledChange: echo.handleEnabledChange,
      onDefaultDeliveryPolicyChange: handleDeliveryPolicyChange,
      preferencesLoading: loading,
      preferencesSaving: preferencesBusy,
      preferencesFeedback: feedback,
      preferencesRecovery: recovery,
    },
    permissions: {
      preferencesFeedback: feedback,
      preferencesRecovery: recovery,
      onPermissionModeChange: handlePermissionModeChange,
      permissionMode:
        preferences.default_agent_options.permission_mode
        ?? DEFAULT_AGENT_PERMISSION_MODE,
      preferencesLoading: loading,
      preferencesSaving: preferencesBusy,
    },
  };
}
