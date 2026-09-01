/**
 * INPUT: 当前运行类型的可用模型目录与 Preferences 版本化保存入口。
 * OUTPUT: 保留最后一次模型目录快照，读取失败可重试，写入结果交由 Preferences 对账。
 * POS: 默认模型选择控制器；不复制或覆盖 Preferences 的 mutation 恢复状态机。
 */
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  useSyncExternalStore,
} from "react";

import { listProviderOptionsApi } from "@/lib/api/settings/provider-api";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  AgentRuntimeKind,
  UserPreferences,
} from "@/types/settings/preferences";

import {
  EMPTY_DEFAULT_MODEL_CATALOG,
  type DefaultModelCatalog,
  type DefaultModelPreferenceRole,
  applyDefaultModelSelection,
  buildDefaultModelCatalog,
  buildDefaultModelPreferencesView,
  decodeDefaultModelValue,
} from "./model/default-model-preferences-model";

interface ProviderCatalogState {
  catalog: DefaultModelCatalog;
  failed: boolean;
  loading: boolean;
  ownerGeneration: number;
  runtimeKind: AgentRuntimeKind | null;
}

interface UseDefaultModelPreferencesOptions {
  agentRuntimeKind: AgentRuntimeKind;
  getCurrentPreferences: () => UserPreferences;
  persistPreferences: (
    preferences: UserPreferences,
  ) => Promise<UserPreferences | null>;
  preferences: UserPreferences;
  preferencesSaving: boolean;
}

const EMPTY_CATALOG: ProviderCatalogState = {
  catalog: EMPTY_DEFAULT_MODEL_CATALOG,
  failed: false,
  loading: true,
  ownerGeneration: -1,
  runtimeKind: null,
};

export function useDefaultModelPreferences({
  agentRuntimeKind,
  getCurrentPreferences,
  persistPreferences,
  preferences,
  preferencesSaving,
}: UseDefaultModelPreferencesOptions) {
  const { t } = useI18n();
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const [catalog, setCatalog] = useState(EMPTY_CATALOG);
  const [catalogReloadKey, setCatalogReloadKey] = useState(0);
  const [savingRole, setSavingRole] =
    useState<DefaultModelPreferenceRole | null>(null);

  useEffect(() => {
    let cancelled = false;
    setCatalog((current) => (
      current.ownerGeneration === ownerGeneration
      && current.runtimeKind === agentRuntimeKind
    )
      ? { ...current, failed: false, loading: true }
      : {
          catalog: EMPTY_DEFAULT_MODEL_CATALOG,
          failed: false,
          loading: true,
          ownerGeneration,
          runtimeKind: agentRuntimeKind,
        });

    void listProviderOptionsApi(agentRuntimeKind)
      .then((result) => {
        if (
          cancelled
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        setCatalog({
          catalog: buildDefaultModelCatalog(result),
          failed: false,
          loading: false,
          ownerGeneration,
          runtimeKind: agentRuntimeKind,
        });
      })
      .catch(() => {
        if (
          !cancelled
          && isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          setCatalog((current) => ({
            ...current,
            failed: true,
            loading: false,
          }));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agentRuntimeKind, catalogReloadKey, ownerGeneration]);

  const subscriptionLabel = t("settings.providers.subscription_badge");
  const view = useMemo(() => buildDefaultModelPreferencesView(
    catalog.catalog,
    preferences,
    subscriptionLabel,
  ), [catalog.catalog, preferences, subscriptionLabel]);

  const handleChange = useCallback((
    value: string,
    role: DefaultModelPreferenceRole,
  ) => {
    const selection = decodeDefaultModelValue(value);
    if (!selection || savingRole || preferencesSaving) {
      return;
    }
    setSavingRole(role);
    const next = applyDefaultModelSelection(
      getCurrentPreferences(),
      role,
      selection,
    );
    void persistPreferences(next)
      .catch(() => {})
      .finally(() => setSavingRole(null));
  }, [
    getCurrentPreferences,
    persistPreferences,
    preferencesSaving,
    savingRole,
  ]);

  return {
    catalogFailed: catalog.failed,
    handleChange,
    loading: catalog.loading,
    options: view.options,
    retryCatalog: () => setCatalogReloadKey((current) => current + 1),
    savingRole,
    values: view.values,
  };
}
