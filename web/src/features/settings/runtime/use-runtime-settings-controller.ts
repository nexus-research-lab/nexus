/**
 * INPUT: Runtime 可用性读取与版本化 Preferences 控制器。
 * OUTPUT: 独立的运行引擎检查反馈和偏好设置动作。
 * POS: Runtime 设置控制器；可用性读取失败不得触发 Preferences 对账。
 */
import { useCallback, useRef, useState } from "react";

import {
  getNxsRuntimeStatusApi,
  inspectSandboxResourcesApi,
  reconcileSandboxResourcesApi,
} from "@/lib/api/settings/runtime-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  DEFAULT_WEB_SEARCH_PROVIDER,
  normalizeAgentRuntimeKind,
  type AgentRuntimeKind,
  type NXSSandboxDiagnosticState,
  type SandboxResourceRecord,
  type WebSearchProvider,
  type WebSearchSettings,
} from "@/types/settings/preferences";

import { useUserPreferences } from "../general/use-user-preferences";
import type { PreferenceFeedback } from "../general/model/settings-preferences-model";

const SANDBOX_RECOVERY_AGE_SECONDS = 60 * 60;

export interface SandboxRecoverySummary {
  candidateCount: number;
  removedCount: number;
  resourceCount: number;
  unknownCount: number;
}

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
  const [sandboxChecking, setSandboxChecking] = useState(false);
  const [sandboxState, setSandboxState] = useState<NXSSandboxDiagnosticState | null>(null);
  const sandboxRequest = useRef(false);
  const onCheckSandbox = useCallback(async () => {
    if (sandboxRequest.current) return;
    sandboxRequest.current = true;
    setSandboxChecking(true);
    setSandboxState(null);
    try {
      const status = await getNxsRuntimeStatusApi(true);
      setSandboxState(status.sandbox?.state ?? "unknown");
    } catch {
      setSandboxState("unknown");
    } finally {
      sandboxRequest.current = false;
      setSandboxChecking(false);
    }
  }, []);
  const [sandboxRecoveryChecking, setSandboxRecoveryChecking] = useState(false);
  const [sandboxRecoveryApplying, setSandboxRecoveryApplying] = useState(false);
  const [sandboxRecoverySummary, setSandboxRecoverySummary] = useState<SandboxRecoverySummary | null>(null);
  const [sandboxRecoveryError, setSandboxRecoveryError] = useState(false);
  const sandboxRecoveryRequest = useRef(false);
  const projectSandboxRecoverySummary = useCallback((
    resourceCount: number,
    resources: SandboxResourceRecord[],
    candidateCount: number,
    removedCount = 0,
  ): SandboxRecoverySummary => ({
    candidateCount,
    removedCount,
    resourceCount,
    unknownCount: resources.filter((resource) => resource.marker.cleanup_state === "cleanup_unknown").length,
  }), []);
  const onInspectSandboxResources = useCallback(async () => {
    if (sandboxRecoveryRequest.current) return;
    sandboxRecoveryRequest.current = true;
    setSandboxRecoveryChecking(true);
    setSandboxRecoveryError(false);
    try {
      const inspection = await inspectSandboxResourcesApi();
      const preview = await reconcileSandboxResourcesApi({
        older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
        apply: false,
      });
      setSandboxRecoverySummary(projectSandboxRecoverySummary(
        inspection.resources.length,
        inspection.resources,
        preview.candidates.length,
      ));
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryChecking(false);
    }
  }, [projectSandboxRecoverySummary]);
  const onReconcileSandboxResources = useCallback(async () => {
    if (sandboxRecoveryRequest.current) return;
    sandboxRecoveryRequest.current = true;
    setSandboxRecoveryApplying(true);
    setSandboxRecoveryError(false);
    try {
      const result = await reconcileSandboxResourcesApi({
        older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
        apply: true,
      });
      const inspection = await inspectSandboxResourcesApi();
      const preview = await reconcileSandboxResourcesApi({
        older_than_seconds: SANDBOX_RECOVERY_AGE_SECONDS,
        apply: false,
      });
      setSandboxRecoverySummary(projectSandboxRecoverySummary(
        inspection.resources.length,
        inspection.resources,
        preview.candidates.length,
        result.removed.length,
      ));
    } catch {
      setSandboxRecoveryError(true);
    } finally {
      sandboxRecoveryRequest.current = false;
      setSandboxRecoveryApplying(false);
    }
  }, [projectSandboxRecoverySummary]);
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
    sandboxChecking, sandboxState, onCheckSandbox,
    sandboxRecoveryApplying,
    sandboxRecoveryChecking,
    sandboxRecoveryError,
    sandboxRecoverySummary,
    onInspectSandboxResources,
    onReconcileSandboxResources,
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
