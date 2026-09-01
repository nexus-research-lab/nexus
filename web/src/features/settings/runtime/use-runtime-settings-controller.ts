/**
 * INPUT: Runtime 可用性读取与版本化 Preferences 控制器。
 * OUTPUT: 独立的运行引擎检查反馈和偏好设置动作。
 * POS: Runtime 设置控制器；可用性读取失败不得触发 Preferences 对账。
 */
import { useCallback, useState } from "react";

import { getNxsRuntimeStatusApi } from "@/lib/api/settings/runtime-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  DEFAULT_WEB_SEARCH_PROVIDER,
  normalizeAgentRuntimeKind,
  type AgentRuntimeKind,
  type WebSearchProvider,
  type WebSearchSettings,
} from "@/types/settings/preferences";

import { useUserPreferences } from "../general/use-user-preferences";
import type { PreferenceFeedback } from "../general/model/settings-preferences-model";

export function useRuntimeSettingsController() {
  const { t } = useI18n();
  const preferencesStore = useUserPreferences();
  const {
    feedback,
    loading,
    preferences,
    recovery,
    saving,
    updatePreferences,
    writable,
  } = preferencesStore;
  const [nxsRuntimeChecking, setNxsRuntimeChecking] = useState(false);
  const [runtimeFeedback, setRuntimeFeedback] =
    useState<PreferenceFeedback | null>(null);
  const runtimeKind = normalizeAgentRuntimeKind(preferences.agent_runtime_kind);
  const preferencesBusy = saving || !writable || nxsRuntimeChecking;

  const selectRuntime = useCallback((value: AgentRuntimeKind) => {
    updatePreferences((current) => ({
      ...current,
      agent_runtime_kind: value,
    }));
  }, [updatePreferences]);

  const verifyAndSelectNxs = useCallback(async () => {
    setNxsRuntimeChecking(true);
    setRuntimeFeedback(null);
    try {
      const status = await getNxsRuntimeStatusApi();
      if (status.available) {
        selectRuntime("nxs");
        return;
      }
      setRuntimeFeedback({
        impact: t("settings.runtime.kernel_check_not_changed_impact"),
        title: t("settings.runtime.kernel_check_failed_title"),
        tone: "error",
      });
    } catch {
      setRuntimeFeedback({
        impact: t("settings.runtime.kernel_check_not_changed_impact"),
        title: t("settings.runtime.kernel_check_failed_title"),
        tone: "error",
      });
    } finally {
      setNxsRuntimeChecking(false);
    }
  }, [selectRuntime, t]);

  const onRuntimeKindChange = useCallback((value: AgentRuntimeKind) => {
    if (value === runtimeKind) {
      return;
    }
    setRuntimeFeedback(null);
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

  const onWebSearchPatch = useCallback((patch: Partial<WebSearchSettings>) => {
    updatePreferences((current) => ({
      ...current,
      web_search: {
        ...(current.web_search ?? { enabled: true, provider: DEFAULT_WEB_SEARCH_PROVIDER }),
        ...patch,
        ...(patch.base_url !== undefined && current.web_search?.provider === "searxng"
          ? { enabled: patch.base_url.trim() !== "" }
          : {}),
      },
    }));
  }, [updatePreferences]);

  const onWebSearchAPIKeyChange = useCallback((value: string) => {
    updatePreferences((current) => {
      const provider = current.web_search?.provider ?? DEFAULT_WEB_SEARCH_PROVIDER;
      return {
        ...current,
        web_search: {
          ...(current.web_search ?? { enabled: true, provider }),
          api_key_configured: false,
          api_key_masked: "",
          enabled: provider === "anysearch" ? true : value.trim() !== "",
        },
        web_search_api_key: value,
      };
    });
  }, [updatePreferences]);

  const onWebSearchProviderChange = useCallback((provider: WebSearchProvider) => {
    updatePreferences((current) => ({
      ...current,
      web_search: {
        ...current.web_search,
        api_key_configured: false,
        api_key_masked: "",
        enabled: provider === DEFAULT_WEB_SEARCH_PROVIDER,
        provider,
        base_url: undefined,
        use_provider_extract: false,
        anysearch: provider === "anysearch" ? current.web_search?.anysearch : undefined,
      },
      web_search_api_key: "",
    }));
  }, [updatePreferences]);

  return {
    preferencesFeedback: feedback,
    preferencesRecovery: recovery,
    runtimeFeedback,
    loading,
    nxsRuntimeChecking,
    onRuntimeKindChange,
    onToolSearchChange,
    onWebSearchAPIKeyChange,
    onWebSearchPatch,
    onWebSearchProviderChange,
    preferencesBusy,
    runtimeKind,
    toolSearchEnabled: preferences.runtime_settings?.nxs?.tool_search === true,
    webSearch: preferences.web_search,
    webSearchAPIKey: preferences.web_search_api_key ?? "",
  };
}
