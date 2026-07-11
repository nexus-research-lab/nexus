import { useCallback, useMemo } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderPreset,
} from "@/types/capability/provider";

import type { ProviderSettingsApi } from "../../provider-settings-api";
import {
  buildProviderEnabledPayload,
  buildProviderPayloadFromDraft,
  getProviderDraftError,
  providerDraftHasChanges,
} from "../../model/provider-config-model";
import { buildProviderErrorFeedback } from "../../model/provider-feedback-model";
import type {
  FeedbackState,
  ProviderDraft,
} from "../../model/provider-settings-types";
import type { RunProviderCommand } from "../use-provider-command";

export interface ProviderPersistResult {
  changed: boolean;
  record: ProviderConfigRecord;
}

export type PersistProvider = (options?: {
  showError?: boolean;
}) => Promise<ProviderPersistResult | null>;

interface UseProviderPersistenceOptions {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isCreating: boolean;
  isEditing: boolean;
  isEmptyMode: boolean;
  providerApi: ProviderSettingsApi;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  updateDraft: (patch: Partial<ProviderDraft>) => void;
  visibilityScope: ProviderConfigRecord["visibility"];
}

export function useProviderPersistence({
  currentPreset,
  draft,
  isCreating,
  isEditing,
  isEmptyMode,
  providerApi,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  t,
  updateDraft,
  visibilityScope,
}: UseProviderPersistenceOptions) {
  const canSave = useMemo(() => (
    !isEmptyMode
    && selectedCanManage
    && getProviderDraftError(draft, currentPreset, isCreating, t) === null
  ), [currentPreset, draft, isCreating, isEmptyMode, selectedCanManage, t]);

  const persistProvider = useCallback<PersistProvider>(async (options) => {
    if (isEmptyMode) {
      return null;
    }
    if (isEditing && selectedRecord?.can_manage === false) {
      return { changed: false, record: selectedRecord };
    }
    const validationError = getProviderDraftError(
      draft,
      currentPreset,
      isCreating,
      t,
    );
    if (validationError) {
      if (options?.showError ?? true) {
        setFeedback({
          tone: "error",
          title: t("settings.providers.config_incomplete_title"),
          message: validationError,
        });
      }
      return null;
    }
    if (
      isEditing
      && !providerDraftHasChanges(draft, selectedRecord, currentPreset)
    ) {
      return selectedRecord ? { changed: false, record: selectedRecord } : null;
    }

    try {
      const payload = buildProviderPayloadFromDraft(draft, currentPreset);
      const authToken = draft.auth_token.trim();
      if (authToken) {
        payload.auth_token = authToken;
      }
      const result = isEditing && selectedRecord
        ? await providerApi.updateConfig(selectedRecord.provider, payload)
        : await providerApi.createConfig({
          ...payload,
          provider: draft.provider.trim(),
          visibility: visibilityScope,
          auth_token: authToken,
          provider_kind: draft.provider_kind,
        });
      return { changed: true, record: result };
    } catch (error) {
      if (options?.showError ?? true) {
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.save_failed_title"),
          t("settings.providers.check_config_retry"),
        ));
      }
      return null;
    }
  }, [
    currentPreset,
    draft,
    isCreating,
    isEditing,
    isEmptyMode,
    providerApi,
    selectedRecord,
    setFeedback,
    t,
    visibilityScope,
  ]);

  const handleProviderFieldBlur = useCallback(() => {
    if (!canSave) {
      return;
    }
    void runCommand({ kind: "save-provider" }, async () => {
      const result = await persistProvider({ showError: false });
      if (result?.changed) {
        await refreshAll(result.record.provider);
      }
    });
  }, [canSave, persistProvider, refreshAll, runCommand]);

  const handleEnabledChange = useCallback((checked: boolean) => {
    if (!selectedCanManage || !selectedRecord) {
      return;
    }
    void runCommand({ kind: "toggle-provider" }, async () => {
      updateDraft({ enabled: checked });
      try {
        const result = await providerApi.updateConfig(
          selectedRecord.provider,
          buildProviderEnabledPayload(
            selectedRecord,
            checked,
            draft.auth_token,
          ),
        );
        await refreshAll(result.provider);
      } catch (error) {
        updateDraft({ enabled: !checked });
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.save_failed_title"),
          t("settings.providers.check_config_retry"),
        ));
      }
    });
  }, [
    draft.auth_token,
    providerApi,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
    updateDraft,
  ]);

  return {
    handleEnabledChange,
    handleProviderFieldBlur,
    persistProvider,
  };
}
