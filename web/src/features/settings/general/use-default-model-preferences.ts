import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { listProviderOptionsApi } from "@/lib/api/provider-config-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ProviderOption } from "@/types/capability/provider";
import type {
  AgentRuntimeKind,
  UserPreferences,
} from "@/types/settings/preferences";

import {
  type DefaultModelPreferenceRole,
  type DefaultModelSelection,
  applyDefaultModelSelection,
  buildDefaultModelOptions,
  decodeDefaultModelValue,
  encodeOptionalModelSelection,
} from "./settings-preferences-model";

interface ProviderCatalogState {
  agentDefault: DefaultModelSelection | null;
  agentOptions: ProviderOption[];
  backgroundOptions: ProviderOption[];
  feedback: string | null;
  imageDefault: DefaultModelSelection | null;
  imageOptions: ProviderOption[];
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
  agentDefault: null,
  agentOptions: [],
  backgroundOptions: [],
  feedback: null,
  imageDefault: null,
  imageOptions: [],
  loading: true,
};

function buildSelection(provider?: string | null, model?: string | null) {
  const normalizedProvider = provider?.trim();
  const normalizedModel = model?.trim();
  return normalizedProvider && normalizedModel
    ? { provider: normalizedProvider, model: normalizedModel }
    : null;
}

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
  const requestSequenceRef = useRef(0);

  useEffect(() => {
    const sequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = sequence;
    let cancelled = false;
    setCatalog((current) => ({ ...current, feedback: null, loading: true }));

    void listProviderOptionsApi(agentRuntimeKind)
      .then((result) => {
        if (cancelled || requestSequenceRef.current !== sequence) {
          return;
        }
        setCatalog({
          agentDefault: buildSelection(
            result.default_provider,
            result.default_model,
          ),
          agentOptions: result.items ?? [],
          backgroundOptions: result.background_items ?? result.items ?? [],
          feedback: null,
          imageDefault: buildSelection(
            result.default_image_provider,
            result.default_image_model,
          ),
          imageOptions: result.image_items ?? [],
          loading: false,
        });
      })
      .catch((error: unknown) => {
        if (!cancelled && requestSequenceRef.current === sequence) {
          setCatalog((current) => ({
            ...current,
            feedback: error instanceof Error
              ? error.message
              : "默认对话模型加载失败",
            loading: false,
          }));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [agentRuntimeKind]);

  const subscriptionLabel = t("settings.providers.subscription_badge");
  const options = useMemo(() => ({
    agent: buildDefaultModelOptions(catalog.agentOptions, subscriptionLabel),
    background: buildDefaultModelOptions(
      catalog.backgroundOptions,
      subscriptionLabel,
    ),
    image: buildDefaultModelOptions(catalog.imageOptions, subscriptionLabel),
  }), [catalog, subscriptionLabel]);
  const values = {
    agent: encodeOptionalModelSelection(
      preferences.default_agent_options.provider || catalog.agentDefault?.provider,
      preferences.default_agent_options.model || catalog.agentDefault?.model,
    ),
    background: encodeOptionalModelSelection(
      preferences.default_background_model_selection?.provider,
      preferences.default_background_model_selection?.model,
    ),
    image: encodeOptionalModelSelection(
      preferences.default_image_model_selection?.provider || catalog.imageDefault?.provider,
      preferences.default_image_model_selection?.model || catalog.imageDefault?.model,
    ),
  };

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
          feedback: error instanceof Error
            ? error.message
            : "默认对话模型保存失败",
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
    options,
    savingRole,
    values,
  };
}
