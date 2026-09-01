"use client";

// INPUT: Composer Session 设置 scope、资源读取与设置修改请求。
// OUTPUT: 保留选择的资源状态、exact Session mutation 失败与显式恢复动作。
// POS: Composer Session-setting 编排边界；读取不清理 unknown，也不自动重放修改。

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  getInitialAgentOptions,
  USER_PREFERENCES_CHANGED_EVENT,
} from "@/config/runtime-options";
import { CAPABILITY_SUMMARY_MUTATED_EVENT } from "@/features/capability/capability-summary-events";
import {
  AGENT_PERMISSION_MODES,
  DEFAULT_AGENT_PERMISSION_MODE,
} from "@/lib/agent-options";
import {
  getSessionRuntimeSettingsApi,
  type SessionRuntimeSettings,
  updateSessionRuntimeSettingsApi,
} from "@/lib/api/conversation/session-api";
import { getConnectorsApi } from "@/lib/api/capability/connector-api";
import { listProviderOptionsApi } from "@/lib/api/settings/provider-api";
import { subscribeSessionRuntimeSettingsUpdated } from "@/lib/conversation/session-runtime-settings-events";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ProviderOptionsResponse } from "@/types/capability/provider";
import type { ConnectorInfo } from "@/types/capability/connector";

import type {
  ComposerSessionSettingsScope,
  ComposerSessionSettingsTarget,
} from "../composer-model";
import {
  buildComposerReadFailure,
  buildComposerSettingsMutationFailure,
  createComposerSettingsMutationIntent,
  isSameComposerSettingsIntent,
  type ComposerReadFailure,
  type ComposerSettingKind,
  type ComposerSettingsMutationFailure,
} from "./composer-settings-reliability";

const EMPTY_SETTINGS: SessionRuntimeSettings = {
  connector_ids: null,
  model: "",
  permission_mode: "",
  provider: "",
};

export function useComposerSessionSettings(
  scope?: ComposerSessionSettingsScope,
) {
  const { t } = useI18n();
  const [selectedTargetId, setSelectedTargetId] = useState(
    scope?.initialTargetId ?? "",
  );
  const [settingsBySession, setSettingsBySession] = useState<
    Record<string, SessionRuntimeSettings>
  >({});
  const [providerOptions, setProviderOptions] =
    useState<ProviderOptionsResponse | null>(null);
  const [providerOptionsLoading, setProviderOptionsLoading] = useState(false);
  const [connectors, setConnectors] = useState<ConnectorInfo[]>([]);
  const [connectorsLoading, setConnectorsLoading] = useState(false);
  const [connectorsFailure, setConnectorsFailure] =
    useState<ComposerReadFailure | null>(null);
  const [loadingSessionKeys, setLoadingSessionKeys] = useState<string[]>([]);
  const [savingSessionKey, setSavingSessionKey] = useState<string | null>(null);
  const [providerFailure, setProviderFailure] =
    useState<ComposerReadFailure | null>(null);
  const [settingsReadFailures, setSettingsReadFailures] = useState<Record<
    string,
    ComposerReadFailure
  >>({});
  const [mutationFailures, setMutationFailures] = useState<Record<
    string,
    ComposerSettingsMutationFailure
  >>({});
  const [preferencesRevision, setPreferencesRevision] = useState(0);
  const [connectorsRevision, setConnectorsRevision] = useState(0);
  const previousInitialTargetRef = useRef(scope?.initialTargetId ?? "");
  const loadingSessionKeysRef = useRef(new Set<string>());
  const savingSessionKeysRef = useRef(new Set<string>());
  const settingsBySessionRef = useRef(settingsBySession);
  const mutationFailuresRef = useRef(mutationFailures);
  const providerOptionsRuntimeKindRef = useRef("");
  const target = scope?.targets.find(
    (candidate) => candidate.agentId === selectedTargetId,
  ) ?? scope?.targets.find(
    (candidate) => candidate.agentId === scope.initialTargetId,
  ) ?? scope?.targets[0];
  const settings = target
    ? settingsBySession[target.sessionKey] ?? EMPTY_SETTINGS
    : EMPTY_SETTINGS;
  useEffect(() => {
    const initialTargetId = scope?.initialTargetId ?? "";
    const initialTargetChanged =
      previousInitialTargetRef.current !== initialTargetId;
    previousInitialTargetRef.current = initialTargetId;
    if (
      initialTargetChanged
      || !scope?.targets.some(
        (candidate) => candidate.agentId === selectedTargetId,
      )
    ) {
      setSelectedTargetId(initialTargetId);
    }
  }, [scope?.initialTargetId, scope?.targets, selectedTargetId]);

  useEffect(() => {
    if (!scope?.runtimeKind) {
      setProviderOptions(null);
      setProviderOptionsLoading(false);
      setProviderFailure(null);
      providerOptionsRuntimeKindRef.current = "";
      return undefined;
    }
    const runtimeKind = scope.runtimeKind;
    let active = true;
    if (providerOptionsRuntimeKindRef.current !== runtimeKind) {
      providerOptionsRuntimeKindRef.current = runtimeKind;
      setProviderOptions(null);
      setProviderFailure(null);
    }
    setProviderOptionsLoading(true);
    void listProviderOptionsApi(runtimeKind)
      .then((result) => {
        if (active) {
          setProviderOptions(result);
          setProviderFailure(null);
        }
      })
      .catch((requestError: unknown) => {
        if (active) {
          setProviderFailure(buildComposerReadFailure(
            requestError,
            "providers",
            t("composer.provider_options_load_failed"),
            t,
          ));
        }
      })
      .finally(() => {
        if (active) {
          setProviderOptionsLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [preferencesRevision, scope?.runtimeKind, t]);

  useEffect(() => {
    if (!scope?.runtimeKind) {
      setConnectors([]);
      setConnectorsLoading(false);
      setConnectorsFailure(null);
      return undefined;
    }
    let active = true;
    setConnectorsLoading(true);
    void getConnectorsApi({ status: "available" })
      .then((items) => {
        if (active) {
          setConnectors(items.filter(
            (connector) => connector.connection_state === "connected",
          ));
          setConnectorsFailure(null);
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setConnectorsFailure(buildComposerReadFailure(
            error,
            "connectors",
            t("composer.connectors_load_failed"),
            t,
          ));
        }
      })
      .finally(() => {
        if (active) setConnectorsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [connectorsRevision, scope?.runtimeKind, t]);

  useEffect(() => {
    const handlePreferencesChange = () => {
      setPreferencesRevision((current) => current + 1);
    };
    window.addEventListener(
      USER_PREFERENCES_CHANGED_EVENT,
      handlePreferencesChange,
    );
    return () => {
      window.removeEventListener(
        USER_PREFERENCES_CHANGED_EVENT,
        handlePreferencesChange,
      );
    };
  }, []);

  useEffect(() => {
    const handleCapabilityMutation = (event: Event) => {
      const detail = (event as CustomEvent<Record<string, unknown>>).detail;
      if (detail?.source === "custom-mcp") {
        setConnectorsRevision((current) => current + 1);
      }
    };
    window.addEventListener(
      CAPABILITY_SUMMARY_MUTATED_EVENT,
      handleCapabilityMutation,
    );
    return () => {
      window.removeEventListener(
        CAPABILITY_SUMMARY_MUTATED_EVENT,
        handleCapabilityMutation,
      );
    };
  }, []);

  const cacheSettings = useCallback((
    sessionKey: string,
    nextSettings: SessionRuntimeSettings,
  ) => {
    setSettingsBySession((current) => {
      const next = {
        ...current,
        [sessionKey]: nextSettings,
      };
      settingsBySessionRef.current = next;
      return next;
    });
  }, []);

  const cacheMutationFailure = useCallback((
    sessionKey: string,
    failure: ComposerSettingsMutationFailure | null,
  ): void => {
    const next = { ...mutationFailuresRef.current };
    if (failure) {
      next[sessionKey] = failure;
    } else {
      delete next[sessionKey];
    }
    mutationFailuresRef.current = next;
    setMutationFailures(next);
  }, []);

  const loadSettings = useCallback(async (
    sessionKey: string,
    force = false,
  ) => {
    if (
      !sessionKey
      || (!force && settingsBySessionRef.current[sessionKey])
      || loadingSessionKeysRef.current.has(sessionKey)
    ) {
      return;
    }
    loadingSessionKeysRef.current.add(sessionKey);
    setLoadingSessionKeys((current) => (
      current.includes(sessionKey) ? current : [...current, sessionKey]
    ));
    try {
      const result = await getSessionRuntimeSettingsApi(sessionKey);
      if (!savingSessionKeysRef.current.has(sessionKey)) {
        cacheSettings(sessionKey, result);
        cacheMutationFailure(sessionKey, null);
      }
      setSettingsReadFailures((current) => {
        const next = { ...current };
        delete next[sessionKey];
        return next;
      });
    } catch (requestError) {
      setSettingsReadFailures((current) => ({
        ...current,
        [sessionKey]: buildComposerReadFailure(
          requestError,
          "session_settings",
          t("composer.session_settings_load_failed"),
          t,
        ),
      }));
    } finally {
      loadingSessionKeysRef.current.delete(sessionKey);
      setLoadingSessionKeys((current) => (
        current.filter((candidate) => candidate !== sessionKey)
      ));
    }
  }, [cacheMutationFailure, cacheSettings, t]);

  useEffect(() => subscribeSessionRuntimeSettingsUpdated((sessionKey) => {
    if (!settingsBySessionRef.current[sessionKey]) {
      return;
    }
    void loadSettings(sessionKey, true);
  }), [loadSettings]);

  useEffect(() => {
    if (target?.sessionKey && !settingsBySession[target.sessionKey]) {
      void loadSettings(target.sessionKey);
    }
  }, [loadSettings, settingsBySession, target?.sessionKey]);

  const updateSettings = useCallback(async (
    setting: ComposerSettingKind,
    next: SessionRuntimeSettings,
  ) => {
    if (!target) {
      return;
    }
    const { sessionKey } = target;
    if (savingSessionKeysRef.current.has(sessionKey)) {
      return;
    }
    const intent = createComposerSettingsMutationIntent(
      sessionKey,
      setting,
      next,
    );
    const currentFailure = mutationFailuresRef.current[sessionKey];
    if (
      currentFailure?.blocksRepeat
      && isSameComposerSettingsIntent(currentFailure.intent, intent)
    ) {
      return;
    }
    const previous =
      settingsBySessionRef.current[sessionKey] ?? EMPTY_SETTINGS;
    cacheMutationFailure(sessionKey, null);
    savingSessionKeysRef.current.add(sessionKey);
    setSavingSessionKey(sessionKey);
    cacheSettings(sessionKey, next);
    try {
      const saved = await updateSessionRuntimeSettingsApi(sessionKey, next);
      cacheSettings(sessionKey, saved);
    } catch (requestError) {
      const failure = buildComposerSettingsMutationFailure(
        requestError,
        intent,
        t,
      );
      if (failure.effect === "not_applied") {
        cacheSettings(sessionKey, previous);
      }
      cacheMutationFailure(sessionKey, failure);
    } finally {
      savingSessionKeysRef.current.delete(sessionKey);
      setSavingSessionKey((current) =>
        current === sessionKey ? null : current
      );
    }
  }, [cacheMutationFailure, cacheSettings, t, target]);

  const selectTarget = useCallback((agentId: string) => {
    setSelectedTargetId(agentId);
  }, []);
  const resetTarget = useCallback(() => {
    setSelectedTargetId(scope?.initialTargetId ?? "");
  }, [scope?.initialTargetId]);
  const ensureTargetsLoaded = useCallback(async () => {
    await Promise.all(
      (scope?.targets ?? []).map(
        (candidate) => loadSettings(candidate.sessionKey),
      ),
    );
  }, [loadSettings, scope?.targets]);

  const inheritedPermission = resolveInheritedPermission(target);
  const inheritedModel = resolveInheritedModelSelection(
    target,
    providerOptions,
  );
  const effectivePermissionMode =
    settings.permission_mode || inheritedPermission;
  const inheritedConnectorIds = target?.defaultConnectorIds ?? [];
  const enabledConnectorIds = settings.connector_ids ?? inheritedConnectorIds;
  const settingsReadFailure = target
    ? settingsReadFailures[target.sessionKey] ?? null
    : null;
  const mutationFailure = target
    ? mutationFailures[target.sessionKey] ?? null
    : null;
  const settingsLoading = Boolean(
    target && loadingSessionKeys.includes(target.sessionKey)
  );
  const sessionBusy = Boolean(
    target
    && (
      loadingSessionKeys.includes(target.sessionKey)
      || savingSessionKey === target.sessionKey
      || settingsReadFailure
    )
  );
  const targetViews = useMemo(() => (
    (scope?.targets ?? []).map((candidate) => {
      const targetSettings =
        settingsBySession[candidate.sessionKey] ?? EMPTY_SETTINGS;
      return {
        busy: loadingSessionKeys.includes(candidate.sessionKey)
          || savingSessionKey === candidate.sessionKey,
        modelLabel: targetSettings.model
          ? resolveModelLabel(
              targetSettings.provider,
              targetSettings.model,
            providerOptions,
          )
          : resolveInheritedModelSelection(candidate, providerOptions).label,
        target: candidate,
      };
    })
  ), [
    loadingSessionKeys,
    providerOptions,
    savingSessionKey,
    scope?.targets,
    settingsBySession,
  ]);
  return {
    busy: sessionBusy,
    connectors,
    connectorsFailure,
    connectorsLoading,
    enabledConnectorIds,
    ensureTargetsLoaded,
    hasModelOverride: Boolean(settings.provider && settings.model),
    hasPermissionOverride: Boolean(settings.permission_mode),
    inheritedModel: inheritedModel.model,
    isDangerousPermission: effectivePermissionMode === "bypassPermissions",
    inheritedPermissionMode: inheritedPermission,
    inheritedProvider: inheritedModel.provider,
    modelBusy: sessionBusy || providerOptionsLoading || Boolean(providerFailure),
    modelLabel: settings.model
      ? resolveModelLabel(settings.provider, settings.model, providerOptions)
      : inheritedModel.label,
    permissionLabel: permissionModeLabel(
      effectivePermissionMode,
      t,
    ),
    providerOptions,
    providerOptionsLoading,
    providerFailure,
    resetTarget,
    saving: savingSessionKey !== null,
    scope,
    selectTarget,
    settings,
    settingsLoading,
    settingsReadFailure,
    mutationFailure,
    target,
    targetViews,
    retryConnectors: () => setConnectorsRevision((current) => current + 1),
    retryProviderOptions: () => setPreferencesRevision((current) => current + 1),
    retrySessionSettings: () => target
      ? loadSettings(target.sessionKey, true)
      : Promise.resolve(),
    resetModel: () => updateSettings("model", {
      ...settings,
      model: "",
      provider: "",
    }),
    resetPermission: () => updateSettings("permission", {
      ...settings,
      permission_mode: "",
    }),
    updateModel: (provider: string, model: string) => updateSettings("model", {
      ...settings,
      model,
      provider,
    }),
    updatePermission: (permissionMode: string) => updateSettings("permission", {
      ...settings,
      permission_mode: permissionMode,
    }),
    toggleConnector: (connectorId: string) => {
      const nextConnectorIds = enabledConnectorIds.includes(connectorId)
        ? enabledConnectorIds.filter((value) => value !== connectorId)
        : [...enabledConnectorIds, connectorId];
      return updateSettings("connectors", {
        ...settings,
        connector_ids: sameStringSet(nextConnectorIds, inheritedConnectorIds)
          ? null
          : nextConnectorIds,
      });
    },
  };
}

export type ComposerSessionSettingsController = ReturnType<
  typeof useComposerSessionSettings
>;

function resolveInheritedPermission(
  target?: ComposerSessionSettingsTarget,
): string {
  const globalMode = getInitialAgentOptions().permission_mode;
  return target?.defaultPermissionMode?.trim()
    || globalMode?.trim()
    || DEFAULT_AGENT_PERMISSION_MODE;
}

interface InheritedModelSelection {
  label: string;
  model: string;
  provider: string;
}

function resolveInheritedModelSelection(
  target: ComposerSessionSettingsTarget | undefined,
  options: ProviderOptionsResponse | null,
): InheritedModelSelection {
  const provider = target?.defaultProvider?.trim();
  const model = target?.defaultModel?.trim();
  if (provider && model) {
    return {
      label: resolveModelLabel(provider, model, options),
      model,
      provider,
    };
  }
  const selection = options?.default_selection;
  if (selection) {
    return {
      label: selection.model_display_name || selection.model || "—",
      model: selection.model,
      provider: selection.provider,
    };
  }
  return {
    label: model || "—",
    model: model || "",
    provider: provider || "",
  };
}

function resolveModelLabel(
  providerValue: string,
  modelValue: string,
  options: ProviderOptionsResponse | null,
): string {
  const provider = options?.items.find(
    (candidate) => candidate.provider === providerValue,
  );
  const model = provider?.models.find(
    (candidate) => candidate.model_id === modelValue,
  );
  return model?.display_name || modelValue || "—";
}

function permissionModeLabel(
  value: string,
  t: ReturnType<typeof useI18n>["t"],
): string {
  const mode = AGENT_PERMISSION_MODES.find(
    (candidate) => candidate.value === value,
  );
  return mode ? t(mode.labelKey) : value || "—";
}

function sameStringSet(left: string[], right: string[]): boolean {
  return left.length === right.length
    && left.every((value) => right.includes(value));
}
