import { useCallback, useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import { getProviderSettingsApi } from "./provider-settings-api";
import { useProviderCommand } from "./actions/use-provider-command";
import { useProviderConfigActions } from "./actions/config/use-provider-config-actions";
import { useProviderModelActions } from "./actions/model/use-provider-model-actions";
import { buildProviderSettingsPresentation } from "./model/provider-settings-presentation";
import type { FeedbackState } from "./model/provider-settings-types";
import { useProviderWorkspace } from "./workspace/use-provider-workspace";

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

  const presentation = useMemo(() => buildProviderSettingsPresentation({
    canSelectNonRuntimeFormat: configActions.canSelectNonRuntimeFormat,
    currentPreset: workspace.currentPreset,
    draft: workspace.draft,
    isEditing,
    presets: workspace.presets,
    providers: workspace.providers,
    selectedRecord: workspace.selectedRecord,
    t,
  }), [
    configActions.canSelectNonRuntimeFormat,
    isEditing,
    t,
    workspace.currentPreset,
    workspace.draft,
    workspace.presets,
    workspace.providers,
    workspace.selectedRecord,
  ]);

  const dismissFeedback = useCallback(() => setFeedback(null), []);
  return {
    state: {
      ...presentation,
      currentPreset: workspace.currentPreset,
      deleteConfirmOpen: configActions.deleteConfirmOpen,
      deleteTargetRecord: configActions.deleteTargetRecord,
      deleteUsageOpen: configActions.deleteUsageOpen,
      draft: workspace.draft,
      feedback,
      isCreating,
      isEditing,
      isEmptyMode,
      loading: workspace.loading,
      pendingAction,
      selectedCanManage,
      selectedProvider: workspace.selectedProvider,
      selectedRecord: workspace.selectedRecord,
    },
    actions: {
      closeDeleteDialog: configActions.closeDeleteDialog,
      dismissFeedback,
      handleApiFormatChange: configActions.handleApiFormatChange,
      handleCreateFromPreset,
      handleDelete: configActions.handleDelete,
      handleEnabledChange: configActions.handleEnabledChange,
      handleProviderFieldBlur: configActions.handleProviderFieldBlur,
      handleProviderDisplayNameChange:
        configActions.handleProviderDisplayNameChange,
      handleProviderKindChange: configActions.handleProviderKindChange,
      handleRequestDeleteProvider:
        configActions.handleRequestDeleteProvider,
      handleSelectProvider,
      updateDraft: workspace.updateDraft,
    },
    modelActions,
  };
}
