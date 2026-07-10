import { useCallback, useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import { getProviderSettingsApi } from "./provider-settings-api";
import { useProviderCommand } from "./actions/use-provider-command";
import { useProviderConfigActions } from "./actions/use-provider-config-actions";
import { useProviderModelActions } from "./actions/use-provider-model-actions";
import { buildProviderCatalog } from "./model/provider-catalog-model";
import { getEffectiveModelsPath, getProviderTitle } from "./model/provider-config-model";
import {
  formatSupportsProviderKind,
  getPresetFormat,
  orderedPresetProviderKinds,
  presetUsesBuiltinEndpoint,
  SUPPORTED_AGENT_API_FORMATS,
  uniquePresetFormats,
} from "./model/provider-preset-model";
import { API_FORMAT_LABELS } from "./model/provider-settings-presentation";
import type { FeedbackState } from "./model/provider-settings-types";
import { useProviderWorkspace } from "./use-provider-workspace";

export function useProviderSettingsController(
  visibilityScope: ProviderConfigRecord["visibility"],
) {
  const { t } = useI18n();
  const providerApi = getProviderSettingsApi(visibilityScope);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const { pendingAction, runCommand } = useProviderCommand();
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
    providerApi,
    runCommand,
    setFeedback,
    t,
    visibilityScope,
  });
  const modelActions = useProviderModelActions({
    apiFormat: workspace.draft.api_format,
    modelApi: providerApi.model,
    refreshAll: workspace.refreshAll,
    persistProvider: configActions.persistProvider,
    runCommand,
    selectedCanManage,
    selectedRecord: workspace.selectedRecord,
    setFeedback,
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

  const providerCatalog = useMemo(
    () => buildProviderCatalog(workspace.providers),
    [workspace.providers],
  );
  const providerKindOptions = useMemo(() => {
    return orderedPresetProviderKinds(workspace.currentPreset).map((kind) => ({
        value: kind,
        label: kind === "image_generation"
          ? t("settings.providers.kind_image_generation")
          : t("settings.providers.kind_llm"),
      }));
  }, [t, workspace.currentPreset]);
  const formatOptions = useMemo(() => {
    return uniquePresetFormats(workspace.currentPreset).map((item) => {
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
      configuredByPreset: providerCatalog.configuredByPreset,
      currentFormat,
      currentPreset: workspace.currentPreset,
      customProviders: providerCatalog.customProviders,
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
