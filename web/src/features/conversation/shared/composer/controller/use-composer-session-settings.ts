"use client";

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
  const [connectorsError, setConnectorsError] = useState<string | null>(null);
  const [loadingSessionKeys, setLoadingSessionKeys] = useState<string[]>([]);
  const [savingSessionKey, setSavingSessionKey] = useState<string | null>(null);
  const [providerError, setProviderError] = useState<string | null>(null);
  const [settingsError, setSettingsError] = useState<string | null>(null);
  const [preferencesRevision, setPreferencesRevision] = useState(0);
  const previousInitialTargetRef = useRef(scope?.initialTargetId ?? "");
  const loadingSessionKeysRef = useRef(new Set<string>());
  const settingsBySessionRef = useRef(settingsBySession);
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
      setProviderError(null);
      return undefined;
    }
    let active = true;
    setProviderOptions(null);
    setProviderOptionsLoading(true);
    setProviderError(null);
    void listProviderOptionsApi(scope.runtimeKind)
      .then((result) => {
        if (active) {
          setProviderOptions(result);
        }
      })
      .catch((requestError: unknown) => {
        if (active) {
          setProviderError(resolveErrorMessage(
            requestError,
            t("composer.session_settings_load_failed"),
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
      setConnectorsError(null);
      return undefined;
    }
    let active = true;
    setConnectorsLoading(true);
    setConnectorsError(null);
    void getConnectorsApi({ status: "available" })
      .then((items) => {
        if (active) {
          setConnectors(items.filter(
            (connector) => connector.connection_state === "connected",
          ));
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setConnectorsError(resolveErrorMessage(
            error,
            t("composer.connectors_load_failed"),
          ));
        }
      })
      .finally(() => {
        if (active) setConnectorsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [scope?.runtimeKind, t]);

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

  const loadSettings = useCallback(async (sessionKey: string) => {
    if (
      !sessionKey
      || settingsBySessionRef.current[sessionKey]
      || loadingSessionKeysRef.current.has(sessionKey)
    ) {
      return;
    }
    loadingSessionKeysRef.current.add(sessionKey);
    setLoadingSessionKeys((current) => (
      current.includes(sessionKey) ? current : [...current, sessionKey]
    ));
    setSettingsError(null);
    try {
      const result = await getSessionRuntimeSettingsApi(sessionKey);
      cacheSettings(sessionKey, result);
    } catch (requestError) {
      setSettingsError(resolveErrorMessage(
        requestError,
        t("composer.session_settings_load_failed"),
      ));
    } finally {
      loadingSessionKeysRef.current.delete(sessionKey);
      setLoadingSessionKeys((current) => (
        current.filter((candidate) => candidate !== sessionKey)
      ));
    }
  }, [cacheSettings, t]);

  useEffect(() => subscribeSessionRuntimeSettingsUpdated((sessionKey) => {
    const current = settingsBySessionRef.current;
    if (!current[sessionKey]) {
      return;
    }
    const next = { ...current };
    delete next[sessionKey];
    settingsBySessionRef.current = next;
    setSettingsBySession(next);
    void loadSettings(sessionKey);
  }), [loadSettings]);

  useEffect(() => {
    if (target?.sessionKey && !settingsBySession[target.sessionKey]) {
      void loadSettings(target.sessionKey);
    }
  }, [loadSettings, settingsBySession, target?.sessionKey]);

  const updateSettings = useCallback(async (
    next: SessionRuntimeSettings,
  ) => {
    if (!target) {
      return;
    }
    const { sessionKey } = target;
    const previous =
      settingsBySessionRef.current[sessionKey] ?? EMPTY_SETTINGS;
    setSettingsError(null);
    setSavingSessionKey(sessionKey);
    cacheSettings(sessionKey, next);
    try {
      const saved = await updateSessionRuntimeSettingsApi(sessionKey, next);
      cacheSettings(sessionKey, saved);
    } catch (requestError) {
      cacheSettings(sessionKey, previous);
      setSettingsError(resolveErrorMessage(
        requestError,
        t("composer.session_settings_save_failed"),
      ));
    } finally {
      setSavingSessionKey((current) =>
        current === sessionKey ? null : current
      );
    }
  }, [cacheSettings, t, target]);
  const selectTarget = useCallback((agentId: string) => {
    setSettingsError(null);
    setSelectedTargetId(agentId);
  }, []);
  const resetTarget = useCallback(() => {
    setSettingsError(null);
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
  const sessionBusy = Boolean(
    target
    && (
      loadingSessionKeys.includes(target.sessionKey)
      || savingSessionKey === target.sessionKey
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
    connectorsError,
    connectorsLoading,
    enabledConnectorIds,
    ensureTargetsLoaded,
    error: settingsError ?? providerError,
    hasModelOverride: Boolean(settings.provider && settings.model),
    hasPermissionOverride: Boolean(settings.permission_mode),
    inheritedModel: inheritedModel.model,
    isDangerousPermission: effectivePermissionMode === "bypassPermissions",
    inheritedPermissionMode: inheritedPermission,
    inheritedProvider: inheritedModel.provider,
    modelBusy: sessionBusy || providerOptionsLoading,
    modelLabel: settings.model
      ? resolveModelLabel(settings.provider, settings.model, providerOptions)
      : inheritedModel.label,
    permissionLabel: permissionModeLabel(
      effectivePermissionMode,
      t,
    ),
    providerOptions,
    resetTarget,
    saving: savingSessionKey !== null,
    scope,
    selectTarget,
    settings,
    target,
    targetViews,
    resetModel: () => updateSettings({
      ...settings,
      model: "",
      provider: "",
    }),
    resetPermission: () => updateSettings({
      ...settings,
      permission_mode: "",
    }),
    updateModel: (provider: string, model: string) => updateSettings({
      ...settings,
      model,
      provider,
    }),
    updatePermission: (permissionMode: string) => updateSettings({
      ...settings,
      permission_mode: permissionMode,
    }),
    toggleConnector: (connectorId: string) => {
      const nextConnectorIds = enabledConnectorIds.includes(connectorId)
        ? enabledConnectorIds.filter((value) => value !== connectorId)
        : [...enabledConnectorIds, connectorId];
      return updateSettings({
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

function resolveErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim()
    ? error.message
    : fallback;
}

function sameStringSet(left: string[], right: string[]): boolean {
  return left.length === right.length
    && left.every((value) => right.includes(value));
}
