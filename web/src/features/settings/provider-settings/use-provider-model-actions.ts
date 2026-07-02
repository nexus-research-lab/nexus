import { useCallback, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import {
  fetch_provider_models_api,
  test_provider_config_api,
  test_provider_model_api,
  update_provider_model_api,
} from "@/lib/api/provider-config-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import {
  AUTO_TEST_MODEL_VALUE,
  FeedbackState,
  ModelOptionsState,
  build_test_model_options,
  filter_provider_models,
  model_options_from_record,
  model_update_payload,
  parse_provider_options,
  sort_models_enabled_first,
} from "./provider-settings-model";

type SaveProviderConfig = (options?: {
  show_error?: boolean;
  show_success?: boolean;
}) => Promise<ProviderConfigRecord | null>;

interface UseProviderModelActionsOptions {
  api_format: ProviderApiFormat;
  pending_action: string | null;
  selected_can_manage: boolean;
  selected_record: ProviderConfigRecord | null;
  set_feedback: Dispatch<SetStateAction<FeedbackState | null>>;
  set_pending_action: Dispatch<SetStateAction<string | null>>;
  save_provider: SaveProviderConfig;
  refresh_all: (preferredProvider?: string | null) => Promise<void>;
  t: I18nContextValue["t"];
}

export function useProviderModelActions({
  api_format: apiFormat,
  pending_action: pendingAction,
  selected_can_manage: selectedCanManage,
  selected_record: selectedRecord,
  set_feedback: setFeedback,
  set_pending_action: setPendingAction,
  save_provider: saveProvider,
  refresh_all: refreshAll,
  t,
}: UseProviderModelActionsOptions) {
  const [modelQuery, setModelQuery] = useState("");
  const [modelOptions, setModelOptions] =
    useState<ModelOptionsState | null>(null);
  const [addModelOpen, setAddModelOpen] = useState(false);
  const [manualModelId, setManualModelId] = useState("");
  const [manualModelEnabled, setManualModelEnabled] = useState(true);

  const filteredModels = useMemo(() => {
    return filter_provider_models(selectedRecord?.models ?? [], modelQuery);
  }, [modelQuery, selectedRecord]);
  const displayedModels = useMemo(
    () => sort_models_enabled_first(filteredModels),
    [filteredModels],
  );
  const testModelOptions = useMemo(() => {
    return build_test_model_options(
      selectedRecord?.models ?? [],
      t("settings.providers.auto_select_model"),
    );
  }, [selectedRecord, t]);
  const manualModelPlaceholder =
    selectedRecord?.models[0]?.model_id ||
    (apiFormat === "anthropic_messages" ? "opus-4.7" : "model-id");

  const resetModelControls = useCallback(() => {
    setModelQuery("");
    setAddModelOpen(false);
    setModelOptions(null);
    setManualModelId("");
    setManualModelEnabled(true);
  }, []);

  const handleFetchModels = useCallback(async () => {
    if (!selectedRecord || pendingAction || !selectedCanManage) {
      return;
    }
    try {
      setPendingAction("fetch");
      const providerRecord = await saveProvider({
        show_error: true,
        show_success: false,
      });
      if (!providerRecord) {
        return;
      }
      const result = await fetch_provider_models_api(providerRecord.provider);
      await refreshAll(providerRecord.provider);
      setFeedback({
        tone: "success",
        title: t("settings.providers.models_synced_title"),
        message: t("settings.providers.models_synced_message", {
          count: result.count,
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.models_sync_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.models_sync_failed_message"),
      });
    } finally {
      setPendingAction(null);
    }
  }, [
    pendingAction,
    refreshAll,
    saveProvider,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    setPendingAction,
    t,
  ]);

  const handleOpenAddModel = useCallback(() => {
    if (!selectedCanManage) {
      return;
    }
    setManualModelId("");
    setManualModelEnabled(true);
    setAddModelOpen(true);
  }, [selectedCanManage]);

  const handleAddModel = useCallback(async () => {
    if (!selectedRecord || pendingAction || !selectedCanManage) {
      return;
    }
    const modelId = manualModelId.trim();
    if (!modelId) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.model_id_required_title"),
        message: t("settings.providers.model_id_required_message"),
      });
      return;
    }
    try {
      setPendingAction(`add-model:${modelId}`);
      await update_provider_model_api(selectedRecord.provider, modelId, {
        enabled: manualModelEnabled,
        is_default: false,
        capabilities_override: {},
        context_window: null,
        max_output_tokens: null,
        provider_options: {},
      });
      setAddModelOpen(false);
      setManualModelId("");
      await refreshAll(selectedRecord.provider);
      setFeedback({
        tone: "success",
        title: t("settings.providers.model_added_title"),
        message: t("settings.providers.model_added_message", {
          model: modelId,
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.model_add_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.model_add_failed_message"),
      });
    } finally {
      setPendingAction(null);
    }
  }, [
    manualModelEnabled,
    manualModelId,
    pendingAction,
    refreshAll,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    setPendingAction,
    t,
  ]);

  const handleTestProvider = useCallback(async () => {
    if (!selectedRecord || pendingAction || !selectedCanManage) {
      return;
    }
    try {
      setPendingAction("test");
      const providerRecord = await saveProvider({
        show_error: true,
        show_success: false,
      });
      if (!providerRecord) {
        return;
      }
      const result = await test_provider_config_api(providerRecord.provider);
      await refreshAll(providerRecord.provider);
      setFeedback({
        tone: result.success ? "success" : "error",
        title: result.success
          ? t("settings.providers.provider_test_passed_title")
          : t("settings.providers.provider_test_failed_title"),
        message: result.success
          ? t("settings.providers.test_model_message", {
              model: result.model || t("settings.providers.auto_model"),
            })
          : result.error || t("settings.providers.connectivity_failed"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.provider_test_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.check_network_auth"),
      });
    } finally {
      setPendingAction(null);
    }
  }, [
    pendingAction,
    refreshAll,
    saveProvider,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    setPendingAction,
    t,
  ]);

  const handleTestModel = useCallback(
    async (modelId: string) => {
      if (!selectedRecord || pendingAction || !selectedCanManage) {
        return;
      }
      const normalizedModelId = modelId.trim();
      if (!normalizedModelId) {
        return;
      }
      try {
        setPendingAction(`test:${normalizedModelId}`);
        const providerRecord = await saveProvider({
          show_error: true,
          show_success: false,
        });
        if (!providerRecord) {
          return;
        }
        const result = await test_provider_model_api(
          providerRecord.provider,
          normalizedModelId,
        );
        await refreshAll(providerRecord.provider);
        setFeedback({
          tone: result.success ? "success" : "error",
          title: result.success
            ? t("settings.providers.model_test_passed_title")
            : t("settings.providers.model_test_failed_title"),
          message: result.success
            ? t("settings.providers.test_model_message", {
                model: result.model || normalizedModelId,
              })
            : result.error || t("settings.providers.connectivity_failed"),
        });
      } catch (error) {
        setFeedback({
          tone: "error",
          title: t("settings.providers.model_test_failed_title"),
          message:
            error instanceof Error
              ? error.message
              : t("settings.providers.check_network_auth_model"),
        });
      } finally {
        setPendingAction(null);
      }
    },
    [
      pendingAction,
      refreshAll,
      saveProvider,
      selectedCanManage,
      selectedRecord,
      setFeedback,
      setPendingAction,
      t,
    ],
  );

  const handleTestSelection = useCallback(
    (value: string) => {
      if (value === AUTO_TEST_MODEL_VALUE) {
        void handleTestProvider();
        return;
      }
      void handleTestModel(value);
    },
    [handleTestModel, handleTestProvider],
  );

  const handleToggleModel = useCallback(
    async (model: ProviderModelRecord, enabled: boolean) => {
      if (!selectedRecord || pendingAction || !selectedCanManage) {
        return;
      }
      if (model.is_default && !enabled) {
        setFeedback({
          tone: "error",
          title: t("settings.providers.default_model_disable_title"),
          message: t("settings.providers.default_model_disable_message", {
            model: model.display_name || model.model_id,
          }),
        });
        return;
      }
      try {
        setPendingAction(`model:${model.model_id}`);
        await update_provider_model_api(
          selectedRecord.provider,
          model.model_id,
          model_update_payload(model, { enabled }),
        );
        await refreshAll(selectedRecord.provider);
      } catch (error) {
        setFeedback({
          tone: "error",
          title: t("settings.providers.model_status_failed_title"),
          message:
            error instanceof Error
              ? error.message
              : t("settings.providers.retry_later"),
        });
      } finally {
        setPendingAction(null);
      }
    },
    [
      pendingAction,
      refreshAll,
      selectedCanManage,
      selectedRecord,
      setFeedback,
      setPendingAction,
      t,
    ],
  );

  const handleSaveModelOptions = useCallback(async () => {
    if (!selectedRecord || !modelOptions || pendingAction || !selectedCanManage) {
      return;
    }
    try {
      setPendingAction(`options:${modelOptions.model.model_id}`);
      const providerOptions = parse_provider_options(
        modelOptions.provider_options_text,
        t("settings.providers.provider_options_json_object"),
      );
      await update_provider_model_api(
        selectedRecord.provider,
        modelOptions.model.model_id,
        model_update_payload(modelOptions.model, {
          capabilities_override: modelOptions.capabilities,
          context_window: modelOptions.context_window.trim()
            ? Number(modelOptions.context_window)
            : null,
          max_output_tokens: modelOptions.max_output_tokens.trim()
            ? Number(modelOptions.max_output_tokens)
            : null,
          provider_options: providerOptions,
        }),
      );
      setModelOptions(null);
      await refreshAll(selectedRecord.provider);
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.providers.model_options_save_failed_title"),
        message:
          error instanceof Error
            ? error.message
            : t("settings.providers.check_json_format"),
      });
    } finally {
      setPendingAction(null);
    }
  }, [
    modelOptions,
    pendingAction,
    refreshAll,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    setPendingAction,
    t,
  ]);

  return {
    add_model_open: addModelOpen,
    displayed_models: displayedModels,
    handle_add_model: handleAddModel,
    handle_fetch_models: handleFetchModels,
    handle_open_add_model: handleOpenAddModel,
    handle_save_model_options: handleSaveModelOptions,
    handle_test_selection: handleTestSelection,
    handle_toggle_model: handleToggleModel,
    manual_model_enabled: manualModelEnabled,
    manual_model_id: manualModelId,
    manual_model_placeholder: manualModelPlaceholder,
    model_options: modelOptions,
    model_query: modelQuery,
    reset_model_controls: resetModelControls,
    set_add_model_open: setAddModelOpen,
    set_manual_model_enabled: setManualModelEnabled,
    set_manual_model_id: setManualModelId,
    set_model_options: setModelOptions,
    set_model_options_from_record: (model: ProviderModelRecord) =>
      setModelOptions(model_options_from_record(model)),
    set_model_query: setModelQuery,
    test_model_options: testModelOptions,
  };
}
