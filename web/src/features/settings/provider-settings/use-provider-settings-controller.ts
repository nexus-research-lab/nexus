import { useCallback, useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderKind,
} from "@/types/capability/provider";

import { getProviderSettingsApi } from "./provider-settings-api";
import {
  API_FORMAT_LABELS,
  type FeedbackState,
  SUPPORTED_AGENT_API_FORMATS,
  formatSupportsProviderKind,
  getEffectiveModelsPath,
  getProviderTitle,
  getPresetFormat,
  presetProviderKinds,
  presetUsesBuiltinEndpoint,
} from "./provider-settings-model";
import { useProviderConfigActions } from "./use-provider-config-actions";
import { useProviderModelActions } from "./use-provider-model-actions";
import { useProviderWorkspace } from "./use-provider-workspace";

const PROVIDER_KIND_ORDER: ProviderKind[] = ["llm", "image_generation"];

export function useProviderSettingsController(
  visibilityScope: ProviderConfigRecord["visibility"],
) {
  const { t } = useI18n();
  const providerApi = getProviderSettingsApi(visibilityScope);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [pendingAction, setPendingAction] = useState<string | null>(null);
  const workspace = useProviderWorkspace({
    listConfigs: providerApi.listConfigs,
    setFeedback,
    t,
    visibilityScope,
  });
  const isEditing = workspace.mode === "edit" && !!workspace.selectedRecord;
  const isCreating = workspace.mode === "create";
  const isEmptyMode = workspace.mode === "empty";
  const selectedCanManage =
    !isEditing || workspace.selectedRecord?.can_manage !== false;
  const configActions = useProviderConfigActions({
    context: {
      currentPreset: workspace.currentPreset,
      draft: workspace.draft,
      isCreating,
      isEditing,
      isEmptyMode,
      providers: workspace.providers,
      refreshAll: workspace.refreshAll,
      selectedCanManage,
      selectedRecord: workspace.selectedRecord,
      updateDraft: workspace.updateDraft,
    },
    pendingAction,
    providerApi,
    setFeedback,
    t,
    visibilityScope,
  });
  const modelActions = useProviderModelActions({
    apiFormat: workspace.draft.api_format,
    modelApi: providerApi.model,
    pendingAction,
    refreshAll: workspace.refreshAll,
    saveProvider: configActions.saveProvider,
    selectedCanManage,
    selectedRecord: workspace.selectedRecord,
    setFeedback,
    setPendingAction,
    t,
  });
  const { resetModelControls } = modelActions;
  const { createFromPreset, selectProvider } = workspace;

  const handleSelectProvider = useCallback((provider: string) => {
    if (selectProvider(provider)) {
      resetModelControls();
    }
  }, [resetModelControls, selectProvider]);

  const handleCreateFromPreset = useCallback((presetKey: string) => {
    createFromPreset(presetKey);
    resetModelControls();
  }, [createFromPreset, resetModelControls]);

  const configuredByPreset = useMemo(() => {
    const result = new Map<string, ProviderConfigRecord>();
    for (const item of workspace.providers) {
      if (
        item.preset_key
        && item.preset_key !== "custom"
        && !result.has(item.preset_key)
      ) {
        result.set(item.preset_key, item);
      }
    }
    return result;
  }, [workspace.providers]);
  const customProviders = useMemo(
    () => workspace.providers.filter((item) => (
      item.preset_key === "custom"
      || !configuredByPreset.has(item.preset_key)
    )),
    [configuredByPreset, workspace.providers],
  );
  const providerKindOptions = useMemo(() => {
    const availableKinds = presetProviderKinds(workspace.currentPreset);
    return PROVIDER_KIND_ORDER
      .filter((kind) => (
        availableKinds.length === 0 || availableKinds.includes(kind)
      ))
      .map((kind) => ({
        value: kind,
        label: kind === "image_generation"
          ? t("settings.providers.kind_image_generation")
          : t("settings.providers.kind_llm"),
      }));
  }, [t, workspace.currentPreset]);
  const formatOptions = useMemo(() => {
    const seen = new Set<ProviderApiFormat>();
    return (workspace.currentPreset?.formats ?? [])
      .filter((item) => {
        if (seen.has(item.api_format)) {
          return false;
        }
        seen.add(item.api_format);
        return true;
      })
      .map((item) => {
        const supported = formatSupportsProviderKind(
          item,
          workspace.draft.provider_kind,
        );
        return {
          value: item.api_format,
          label: supported || configActions.canSelectNonRuntimeFormat
            ? API_FORMAT_LABELS[item.api_format]
            : `${API_FORMAT_LABELS[item.api_format]}${t("settings.providers.unsupported_suffix")}`,
          disabled:
            !supported && !configActions.canSelectNonRuntimeFormat,
        };
      });
  }, [
    configActions.canSelectNonRuntimeFormat,
    t,
    workspace.currentPreset,
    workspace.draft.provider_kind,
  ]);
  const usesBuiltinEndpoint = presetUsesBuiltinEndpoint(workspace.currentPreset);
  const currentFormat = getPresetFormat(
    workspace.currentPreset,
    workspace.draft.api_format,
  );
  const isApiFormatConfigurable = (
    !!currentFormat
    && formatSupportsProviderKind(
      currentFormat,
      workspace.draft.provider_kind,
    )
  ) || configActions.canSelectNonRuntimeFormat;

  const dismissFeedback = useCallback(() => setFeedback(null), []);
  const reportDefaultModelDisable = useCallback((modelName: string) => {
    setFeedback({
      tone: "error",
      title: t("settings.providers.default_model_disable_title"),
      message: t("settings.providers.default_model_disable_message", {
        model: modelName,
      }),
    });
  }, [t]);

  return {
    state: {
      builtinEndpointFormats: usesBuiltinEndpoint
        ? workspace.currentPreset?.formats ?? []
        : [],
      configuredByPreset,
      currentFormat,
      currentPreset: workspace.currentPreset,
      customProviders,
      deleteConfirmOpen: configActions.deleteConfirmOpen,
      deleteTargetRecord: configActions.deleteTargetRecord,
      deleteUsageOpen: configActions.deleteUsageOpen,
      detailTitle: isEditing && workspace.selectedRecord
        ? getProviderTitle(workspace.selectedRecord)
        : workspace.draft.display_name
          || workspace.currentPreset?.display_name
          || t("settings.providers.custom_provider"),
      draft: workspace.draft,
      feedback,
      formatOptions,
      hasModelsEndpoint: !!getEffectiveModelsPath(
        workspace.draft,
        workspace.currentPreset,
      ).trim(),
      isApiFormatConfigurable,
      isCreating,
      isEditing,
      isEmptyMode,
      loading: workspace.loading,
      pendingAction,
      presetSidebarItems: workspace.presets.filter(
        (preset) => preset.preset_key !== "custom",
      ),
      providerKindOptions,
      selectedCanManage,
      selectedProvider: workspace.selectedProvider,
      selectedRecord: workspace.selectedRecord,
      showProviderShapeControls: workspace.draft.preset_key === "custom",
      showRuntimeFormatBadge:
        workspace.draft.provider_kind === "llm"
        && !SUPPORTED_AGENT_API_FORMATS.has(workspace.draft.api_format),
      submitting: configActions.submitting,
      usesBuiltinEndpoint,
    },
    actions: {
      closeDeleteDialog: configActions.closeDeleteDialog,
      dismissFeedback,
      handleApiFormatChange: configActions.handleApiFormatChange,
      handleCreateFromPreset,
      handleDelete: configActions.handleDelete,
      handleEnabledChange: configActions.handleEnabledChange,
      handleProviderFieldBlur: configActions.handleProviderFieldBlur,
      handleProviderKindChange: configActions.handleProviderKindChange,
      handleRequestDeleteProvider:
        configActions.handleRequestDeleteProvider,
      handleSelectProvider,
      reportDefaultModelDisable,
      updateDraft: workspace.updateDraft,
    },
    modelActions,
  };
}
