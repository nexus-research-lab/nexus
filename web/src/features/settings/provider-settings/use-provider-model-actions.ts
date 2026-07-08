import { useCallback, useMemo, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import {
  fetchProviderModelsApi,
  testProviderConfigApi,
  testProviderModelApi,
  updateProviderModelApi,
} from "@/lib/api/provider-config-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  FetchProviderModelsResponse,
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderModelRecord,
  ProviderTestResult,
  UpdateProviderModelPayload,
} from "@/types/capability/provider";

import {
  AUTO_TEST_MODEL_VALUE,
  FeedbackState,
  ModelOptionsState,
  buildTestModelOptions,
  filterProviderModels,
  modelOptionsFromRecord,
  modelUpdatePayload,
  parseProviderOptions,
  sortModelsEnabledFirst,
} from "./provider-settings-model";

type SaveProviderConfig = (options?: {
  showError?: boolean;
  showSuccess?: boolean;
}) => Promise<ProviderConfigRecord | null>;

export interface ProviderModelActionsApi {
  fetchModels: (provider: string) => Promise<FetchProviderModelsResponse>;
  updateModel: (
    provider: string,
    modelId: string,
    payload: UpdateProviderModelPayload,
  ) => Promise<ProviderModelRecord>;
  testProvider: (provider: string) => Promise<ProviderTestResult>;
  testModel: (provider: string, modelId: string) => Promise<ProviderTestResult>;
}

interface UseProviderModelActionsOptions {
  apiFormat: ProviderApiFormat;
  modelApi?: ProviderModelActionsApi;
  pendingAction: string | null;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  setPendingAction: Dispatch<SetStateAction<string | null>>;
  saveProvider: SaveProviderConfig;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  t: I18nContextValue["t"];
}

const DEFAULT_PROVIDER_MODEL_API: ProviderModelActionsApi = {
  fetchModels: fetchProviderModelsApi,
  updateModel: updateProviderModelApi,
  testProvider: testProviderConfigApi,
  testModel: testProviderModelApi,
};

export function useProviderModelActions({
  apiFormat,
  modelApi = DEFAULT_PROVIDER_MODEL_API,
  pendingAction,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  setPendingAction,
  saveProvider,
  refreshAll,
  t,
}: UseProviderModelActionsOptions) {
  const [modelQuery, setModelQuery] = useState("");
  const [modelOptions, setModelOptions] =
    useState<ModelOptionsState | null>(null);
  const [addModelOpen, setAddModelOpen] = useState(false);
  const [manualModelId, setManualModelId] = useState("");
  const [manualModelEnabled, setManualModelEnabled] = useState(true);

  const filteredModels = useMemo(() => {
    return filterProviderModels(selectedRecord?.models ?? [], modelQuery);
  }, [modelQuery, selectedRecord]);
  const displayedModels = useMemo(
    () => sortModelsEnabledFirst(filteredModels),
    [filteredModels],
  );
  const testModelOptions = useMemo(() => {
    return buildTestModelOptions(
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
        showError: true,
        showSuccess: false,
      });
      if (!providerRecord) {
        return;
      }
      const result = await modelApi.fetchModels(providerRecord.provider);
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
    modelApi,
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
      await modelApi.updateModel(selectedRecord.provider, modelId, {
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
    modelApi,
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
        showError: true,
        showSuccess: false,
      });
      if (!providerRecord) {
        return;
      }
      const result = await modelApi.testProvider(providerRecord.provider);
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
    modelApi,
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
          showError: true,
          showSuccess: false,
        });
        if (!providerRecord) {
          return;
        }
        const result = await modelApi.testModel(
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
      modelApi,
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
        await modelApi.updateModel(
          selectedRecord.provider,
          model.model_id,
          modelUpdatePayload(model, { enabled }),
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
      modelApi,
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
      const providerOptions = parseProviderOptions(
        modelOptions.provider_options_text,
        t("settings.providers.provider_options_json_object"),
      );
      await modelApi.updateModel(
        selectedRecord.provider,
        modelOptions.model.model_id,
        modelUpdatePayload(modelOptions.model, {
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
    modelApi,
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
    addModelOpen,
    displayedModels,
    handleAddModel,
    handleFetchModels,
    handleOpenAddModel,
    handleSaveModelOptions,
    handleTestSelection,
    handleToggleModel,
    manualModelEnabled,
    manualModelId,
    manualModelPlaceholder,
    modelOptions,
    modelQuery,
    resetModelControls,
    setAddModelOpen,
    setManualModelEnabled,
    setManualModelId,
    setModelOptions,
    setModelOptionsFromRecord: (model: ProviderModelRecord) =>
      setModelOptions(modelOptionsFromRecord(model)),
    setModelQuery,
    testModelOptions,
  };
}
