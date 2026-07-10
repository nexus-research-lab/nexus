import { useCallback, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
  ProviderPreset,
} from "@/types/capability/provider";

import type { ProviderSettingsApi } from "../provider-settings-api";
import {
  buildProviderEnabledPayload,
  buildProviderPayloadFromDraft,
  getProviderDraftError,
  getProviderTitle,
  isCustomProviderRecord,
  providerDraftHasChanges,
} from "../model/provider-config-model";
import { buildProviderErrorFeedback } from "../model/provider-feedback-model";
import {
  DEFAULT_AGENT_API_FORMAT,
  formatSupportsProviderKind,
  getPresetFormat,
  getSupportedPresetFormat,
  presetAllowsNonRuntimeConfig,
} from "../model/provider-preset-model";
import type {
  FeedbackState,
  ProviderDraft,
} from "../model/provider-settings-types";
import type { RunProviderCommand } from "./use-provider-command";

interface ProviderConfigContext {
  currentPreset: ProviderPreset | null;
  draft: ProviderDraft;
  isCreating: boolean;
  isEditing: boolean;
  isEmptyMode: boolean;
  providers: ProviderConfigRecord[];
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  updateDraft: (patch: Partial<ProviderDraft>) => void;
}

interface UseProviderConfigActionsOptions {
  context: ProviderConfigContext;
  providerApi: ProviderSettingsApi;
  runCommand: RunProviderCommand;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  visibilityScope: ProviderConfigRecord["visibility"];
}

type DeleteDialogState = {
  kind: "confirm" | "usage";
  provider: string;
} | null;

export interface ProviderPersistResult {
  changed: boolean;
  record: ProviderConfigRecord;
}

export type PersistProvider = (options?: {
  showError?: boolean;
}) => Promise<ProviderPersistResult | null>;

export function useProviderConfigActions({
  context,
  providerApi,
  runCommand,
  setFeedback,
  t,
  visibilityScope,
}: UseProviderConfigActionsOptions) {
  const {
    currentPreset,
    draft,
    isCreating,
    isEditing,
    isEmptyMode,
    providers,
    refreshAll,
    selectedCanManage,
    selectedRecord,
    updateDraft,
  } = context;
  const [deleteDialog, setDeleteDialog] = useState<DeleteDialogState>(null);
  const deleteTargetRecord = useMemo(
    () => providers.find((item) => item.provider === deleteDialog?.provider)
      ?? null,
    [deleteDialog?.provider, providers],
  );
  const canSelectNonRuntimeFormat = draft.provider_kind === "llm"
    && presetAllowsNonRuntimeConfig(currentPreset);
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

  const handleProviderKindChange = useCallback((value: string) => {
    const providerKind = value as ProviderKind;
    const currentFormat = getPresetFormat(currentPreset, draft.api_format);
    const format = currentFormat
      && formatSupportsProviderKind(currentFormat, providerKind)
      ? currentFormat
      : getSupportedPresetFormat(currentPreset, providerKind);
    updateDraft({
      provider_kind: providerKind,
      api_format: format?.api_format ?? (
        providerKind === "image_generation"
          ? "chat_completions"
          : DEFAULT_AGENT_API_FORMAT
      ),
      base_url: format?.base_url ?? draft.base_url,
      models_path: format?.models_path ?? draft.models_path,
    });
  }, [currentPreset, draft, updateDraft]);

  const handleApiFormatChange = useCallback((value: string) => {
    const apiFormat = value as ProviderApiFormat;
    const format = getPresetFormat(currentPreset, apiFormat);
    const supported = !!format
      && formatSupportsProviderKind(format, draft.provider_kind);
    if (!supported && !canSelectNonRuntimeFormat) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.api_format_unsupported_title"),
        message: t("settings.providers.api_format_unsupported_message"),
      });
      return;
    }
    updateDraft({
      api_format: apiFormat,
      base_url: format?.base_url ?? draft.base_url,
      models_path: format?.models_path ?? draft.models_path,
    });
  }, [
    canSelectNonRuntimeFormat,
    currentPreset,
    draft.base_url,
    draft.models_path,
    draft.provider_kind,
    setFeedback,
    t,
    updateDraft,
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

  const handleRequestDeleteProvider = useCallback((item: ProviderConfigRecord) => {
    if (!isCustomProviderRecord(item)) {
      return;
    }
    setDeleteDialog({
      kind: item.usage_count > 0 ? "usage" : "confirm",
      provider: item.provider,
    });
  }, []);

  const handleDelete = useCallback((force = false) => {
    if (!deleteTargetRecord) {
      return;
    }
    if (deleteTargetRecord.usage_count > 0 && !force) {
      setDeleteDialog({
        kind: "usage",
        provider: deleteTargetRecord.provider,
      });
      return;
    }
    void runCommand({ kind: "delete-provider" }, async () => {
      try {
        const result = await providerApi.deleteConfig(
          deleteTargetRecord.provider,
          { force },
        );
        setDeleteDialog(null);
        await refreshAll();
        setFeedback({
          tone: "success",
          title: t("settings.providers.deleted_title"),
          message: result.replacement_provider
            ? t("settings.providers.delete_reassigned_message", {
              count: result.reassigned_runtime_count ?? 0,
              provider: result.replacement_provider,
            })
            : t("settings.providers.delete_removed_message", {
              name: getProviderTitle(deleteTargetRecord),
            }),
        });
      } catch (error) {
        setDeleteDialog(null);
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.delete_failed_title"),
          t("settings.providers.delete_in_use_fallback"),
        ));
      }
    });
  }, [
    deleteTargetRecord,
    providerApi,
    refreshAll,
    runCommand,
    setFeedback,
    t,
  ]);

  return {
    canSelectNonRuntimeFormat,
    closeDeleteDialog: () => setDeleteDialog(null),
    deleteConfirmOpen: deleteDialog?.kind === "confirm",
    deleteTargetRecord,
    deleteUsageOpen: deleteDialog?.kind === "usage",
    handleApiFormatChange,
    handleDelete,
    handleEnabledChange,
    handleProviderFieldBlur,
    handleProviderKindChange,
    handleRequestDeleteProvider,
    persistProvider,
  };
}
