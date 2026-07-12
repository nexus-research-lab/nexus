import { useCallback, useEffect, useMemo, useState } from "react";

import { listProviderOptionsApi } from "@/lib/api/settings/provider-api";
import { getErrorMessage } from "@/lib/error-message";
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
  feedback: string | null;
  loading: boolean;
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
  feedback: null,
  loading: true,
};

export function useDefaultModelPreferences({
  agentRuntimeKind,
  getCurrentPreferences,
  persistPreferences,
  preferences,
  preferencesSaving,
}: UseDefaultModelPreferencesOptions) {
  const { t } = useI18n();
  const [catalog, setCatalog] = useState(EMPTY_CATALOG);
  const [savingRole, setSavingRole] =
    useState<DefaultModelPreferenceRole | null>(null);

  useEffect(() => {
    let cancelled = false;
    setCatalog((current) => ({ ...current, feedback: null, loading: true }));

    void listProviderOptionsApi(agentRuntimeKind)
      .then((result) => {
        if (cancelled) {
          return;
        }
        setCatalog({
          catalog: buildDefaultModelCatalog(result),
          feedback: null,
          loading: false,
        });
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setCatalog((current) => ({
            ...current,
            feedback: getErrorMessage(error, "默认对话模型加载失败"),
            loading: false,
          }));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agentRuntimeKind]);

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
    setCatalog((current) => ({ ...current, feedback: null }));
    const next = applyDefaultModelSelection(
      getCurrentPreferences(),
      role,
      selection,
    );
    void persistPreferences(next)
      .catch((error: unknown) => {
        setCatalog((current) => ({
          ...current,
          feedback: getErrorMessage(error, "默认对话模型保存失败"),
        }));
      })
      .finally(() => setSavingRole(null));
  }, [
    getCurrentPreferences,
    persistPreferences,
    preferencesSaving,
    savingRole,
  ]);

  return {
    feedbackMessage: catalog.feedback,
    handleChange,
    loading: catalog.loading,
    options: view.options,
    savingRole,
    values: view.values,
  };
}
