import { useCallback, useState } from "react";

import { getNxsRuntimeStatusApi } from "@/lib/api/settings/runtime-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  normalizeAgentRuntimeKind,
  type AgentRuntimeKind,
} from "@/types/settings/preferences";

import { useUserPreferences } from "../general/use-user-preferences";

export function useRuntimeSettingsController() {
  const { t } = useI18n();
  const preferencesStore = useUserPreferences();
  const {
    feedback,
    loading,
    preferences,
    saving,
    setFeedback,
    updatePreferences,
  } = preferencesStore;
  const [nxsRuntimeChecking, setNxsRuntimeChecking] = useState(false);
  const runtimeKind = normalizeAgentRuntimeKind(preferences.agent_runtime_kind);
  const preferencesBusy = saving || nxsRuntimeChecking;

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
          || t("settings.runtime.kernel_nxs_unavailable"),
      });
    } catch (error) {
      setFeedback({
        message: getErrorMessage(
          error,
          t("settings.runtime.kernel_check_failed"),
        ),
      });
    } finally {
      setNxsRuntimeChecking(false);
    }
  }, [selectRuntime, setFeedback, t]);

  const onRuntimeKindChange = useCallback((value: AgentRuntimeKind) => {
    if (value === runtimeKind) {
      return;
    }
    if (value === "nxs") {
      void verifyAndSelectNxs();
      return;
    }
    selectRuntime(value);
  }, [runtimeKind, selectRuntime, verifyAndSelectNxs]);

  const onToolSearchChange = useCallback((checked: boolean) => {
    updatePreferences((current) => ({
      ...current,
      runtime_settings: {
        ...current.runtime_settings,
        nxs: {
          ...current.runtime_settings?.nxs,
          tool_search: checked,
        },
      },
    }));
  }, [updatePreferences]);

  return {
    feedbackMessage: feedback?.message,
    loading,
    nxsRuntimeChecking,
    onRuntimeKindChange,
    onToolSearchChange,
    preferencesBusy,
    runtimeKind,
    toolSearchEnabled: preferences.runtime_settings?.nxs?.tool_search === true,
  };
}
