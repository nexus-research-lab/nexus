import { useCallback, useMemo, useState } from "react";

import { setUserPreferences } from "@/config/runtime-options";
import { getUserPreferencesApi } from "@/lib/api/settings/preferences-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  CCSwitchSyncResult,
  ProviderConfigRecord,
} from "@/types/capability/provider";

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
  const { createFromPreset, refreshAll, selectProvider } = workspace;

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
  const reconcileFeedback = useCallback(async () => {
    const previousFeedback = feedback;
    const keepsUnconfirmedMutation = previousFeedback?.mutationEffect === "accepted"
      || previousFeedback?.mutationEffect === "unknown";
    if (!await refreshAll(workspace.selectedProvider)) {
      if (keepsUnconfirmedMutation && previousFeedback) {
        setFeedback({
          ...previousFeedback,
          message: `${previousFeedback.message} ${t("settings.providers.latest_state_refresh_failed_message")}`,
        });
      }
      return;
    }
    if (keepsUnconfirmedMutation && previousFeedback) {
      setFeedback({
        impact: t("settings.providers.latest_state_unconfirmed_impact"),
        message: t("settings.providers.latest_state_unconfirmed_message"),
        mutationEffect: previousFeedback.mutationEffect,
        nextStep: t("settings.providers.latest_state_unconfirmed_next_step"),
        tone: "warning",
        title: t("settings.providers.latest_state_loaded_title"),
      });
      return;
    }
    setFeedback({
      message: t("settings.providers.latest_state_loaded_message"),
      title: t("settings.providers.latest_state_loaded_title"),
      tone: "success",
    });
  }, [feedback, refreshAll, t, workspace.selectedProvider]);
  const handleCCSwitchSynced = useCallback(async (result: CCSwitchSyncResult) => {
    if (result.default_selection) {
      setUserPreferences(await getUserPreferencesApi());
    }
    setFeedback({
      tone: "success",
      title: t("settings.providers.ccswitch_sync_success_title"),
      message: t("settings.providers.ccswitch_sync_success_message", {
        models: result.model_count,
        providers: result.provider_count,
      }),
    });
    await refreshAll(result.default_selection?.provider ?? null);
  }, [refreshAll, t]);
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
      reconcileFeedback,
      handleApiFormatChange: configActions.handleApiFormatChange,
      handleCCSwitchSynced,
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
