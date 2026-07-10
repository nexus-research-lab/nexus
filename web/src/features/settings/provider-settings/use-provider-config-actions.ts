import { useCallback, useMemo, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
  ProviderPreset,
} from "@/types/capability/provider";

import type { ProviderSettingsApi } from "./provider-settings-api";
import {
  DEFAULT_AGENT_API_FORMAT,
  type FeedbackState,
  type ProviderDraft,
  buildProviderPayloadFromDraft,
  formatSupportsProviderKind,
  getProviderDraftError,
  getProviderTitle,
  getPresetFormat,
  getSupportedPresetFormat,
  isCustomProviderRecord,
  presetAllowsNonRuntimeConfig,
  providerDraftHasChanges,
} from "./provider-settings-model";

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
  pendingAction: string | null;
  providerApi: ProviderSettingsApi;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
  visibilityScope: ProviderConfigRecord["visibility"];
}

type DeleteDialogState = {
  kind: "confirm" | "usage";
  provider: string;
} | null;

interface SaveOptions {
  draftOverrides?: Partial<ProviderDraft>;
  showError?: boolean;
  showSuccess?: boolean;
}

function errorFeedback(
  error: unknown,
  title: string,
  fallbackMessage: string,
): FeedbackState {
  return {
    tone: "error",
    title,
    message: error instanceof Error ? error.message : fallbackMessage,
  };
}

export function useProviderConfigActions({
  context,
  pendingAction,
  providerApi,
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
  const [submitting, setSubmitting] = useState(false);
  const [deleteDialog, setDeleteDialog] = useState<DeleteDialogState>(null);
  const savePromiseRef = useRef<Promise<ProviderConfigRecord | null> | null>(null);
  const deleteTargetRecord = useMemo(
    () => providers.find((item) => item.provider === deleteDialog?.provider) ?? null,
    [deleteDialog?.provider, providers],
  );
  const canSelectNonRuntimeFormat =
    draft.provider_kind === "llm"
    && presetAllowsNonRuntimeConfig(currentPreset);
  const canSave = useMemo(() => {
    if (isEmptyMode || !selectedCanManage) {
      return false;
    }
    return getProviderDraftError(
      draft,
      currentPreset,
      isCreating,
      t,
    ) === null;
  }, [currentPreset, draft, isCreating, isEmptyMode, selectedCanManage, t]);

  const saveProvider = useCallback(async (
    options?: SaveOptions,
  ): Promise<ProviderConfigRecord | null> => {
    if (isEmptyMode) {
      return null;
    }
    if (isEditing && selectedRecord?.can_manage === false) {
      return selectedRecord;
    }
    if (savePromiseRef.current) {
      return savePromiseRef.current;
    }
    const nextDraft = { ...draft, ...options?.draftOverrides };
    const showError = options?.showError ?? true;
    const showSuccess = options?.showSuccess ?? false;
    const validationError = getProviderDraftError(
      nextDraft,
      currentPreset,
      isCreating,
      t,
    );
    if (validationError) {
      if (showError) {
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
      && !providerDraftHasChanges(nextDraft, selectedRecord, currentPreset)
    ) {
      return selectedRecord;
    }

    const savePromise = (async () => {
      setSubmitting(true);
      try {
        const payload = buildProviderPayloadFromDraft(nextDraft, currentPreset);
        const authToken = nextDraft.auth_token.trim();
        if (authToken) {
          payload.auth_token = authToken;
        }
        const result = isEditing && selectedRecord
          ? await providerApi.updateConfig(selectedRecord.provider, payload)
          : await providerApi.createConfig({
            ...payload,
            provider: nextDraft.provider.trim(),
            visibility: visibilityScope,
            auth_token: authToken,
            provider_kind: nextDraft.provider_kind,
          });
        await refreshAll(result.provider);
        if (showSuccess) {
          setFeedback({
            tone: "success",
            title: t("settings.providers.saved_title"),
            message: t("settings.providers.saved_message", {
              name: result.display_name || result.provider,
            }),
          });
        }
        return result;
      } catch (error) {
        if (showError) {
          setFeedback(errorFeedback(
            error,
            t("settings.providers.save_failed_title"),
            t("settings.providers.check_config_retry"),
          ));
        }
        return null;
      } finally {
        setSubmitting(false);
      }
    })();

    savePromiseRef.current = savePromise;
    try {
      return await savePromise;
    } finally {
      if (savePromiseRef.current === savePromise) {
        savePromiseRef.current = null;
      }
    }
  }, [
    currentPreset,
    draft,
    isCreating,
    isEditing,
    isEmptyMode,
    providerApi,
    refreshAll,
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
    if (!canSave || pendingAction || submitting) {
      return;
    }
    if (
      isEditing
      && !providerDraftHasChanges(draft, selectedRecord, currentPreset)
    ) {
      return;
    }
    void saveProvider({ showError: false, showSuccess: false });
  }, [
    canSave,
    currentPreset,
    draft,
    isEditing,
    pendingAction,
    saveProvider,
    selectedRecord,
    submitting,
  ]);

  const handleEnabledChange = useCallback((checked: boolean) => {
    if (
      !selectedCanManage
      || !selectedRecord
      || submitting
      || pendingAction
    ) {
      return;
    }
    updateDraft({ enabled: checked });
    void (async () => {
      setSubmitting(true);
      try {
        const payload = {
          provider_kind: selectedRecord.provider_kind,
          preset_key: selectedRecord.preset_key,
          api_format: selectedRecord.api_format,
          display_name: selectedRecord.display_name || selectedRecord.provider,
          base_url: selectedRecord.base_url,
          models_path: selectedRecord.models_path || "",
          enabled: checked,
        };
        const authToken = draft.auth_token.trim();
        const result = await providerApi.updateConfig(
          selectedRecord.provider,
          checked
            ? (authToken ? { ...payload, auth_token: authToken } : payload)
            : { ...payload, auth_token: "" },
        );
        await refreshAll(result.provider);
      } catch (error) {
        updateDraft({ enabled: !checked });
        setFeedback(errorFeedback(
          error,
          t("settings.providers.save_failed_title"),
          t("settings.providers.check_config_retry"),
        ));
      } finally {
        setSubmitting(false);
      }
    })();
  }, [
    draft.auth_token,
    pendingAction,
    providerApi,
    refreshAll,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    submitting,
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

  const handleDelete = useCallback(async (force = false) => {
    if (!deleteTargetRecord || submitting) {
      return;
    }
    if (deleteTargetRecord.usage_count > 0 && !force) {
      setDeleteDialog({
        kind: "usage",
        provider: deleteTargetRecord.provider,
      });
      return;
    }
    try {
      setSubmitting(true);
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
      setFeedback(errorFeedback(
        error,
        t("settings.providers.delete_failed_title"),
        t("settings.providers.delete_in_use_fallback"),
      ));
    } finally {
      setSubmitting(false);
    }
  }, [
    deleteTargetRecord,
    providerApi,
    refreshAll,
    setFeedback,
    submitting,
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
    saveProvider,
    submitting,
  };
}
