import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import type { ProviderModelApi } from "../provider-settings-api";
import { getProviderErrorMessage } from "../model/provider-feedback-model";
import {
  modelUpdatePayload,
  parseProviderOptions,
} from "../model/provider-model-model";
import type {
  FeedbackState,
  ModelOptionsState,
} from "../model/provider-settings-types";
import type { PersistProvider } from "./config/use-provider-persistence";
import type { RunProviderCommand } from "./use-provider-command";

interface UseProviderModelMutationsOptions {
  manualModelEnabled: boolean;
  manualModelId: string;
  modelApi: ProviderModelApi;
  modelOptions: ModelOptionsState | null;
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setAddModelOpen: Dispatch<SetStateAction<boolean>>;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  setManualModelId: Dispatch<SetStateAction<string>>;
  setModelOptions: Dispatch<SetStateAction<ModelOptionsState | null>>;
  t: I18nContextValue["t"];
}

export function useProviderModelMutations({
  manualModelEnabled,
  manualModelId,
  modelApi,
  modelOptions,
  persistProvider,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setAddModelOpen,
  setFeedback,
  setManualModelId,
  setModelOptions,
  t,
}: UseProviderModelMutationsOptions) {
  const handleFetchModels = useCallback(() => {
    if (!selectedRecord || !selectedCanManage) {
      return;
    }
    void runCommand({ kind: "fetch-models" }, async () => {
      let targetProvider: string | null = null;
      let outcome: FeedbackState | null = null;
      try {
        const persisted = await persistProvider({ showError: true });
        if (!persisted) {
          return;
        }
        targetProvider = persisted.record.provider;
        const result = await modelApi.fetchModels(persisted.record.provider);
        outcome = {
          tone: "success",
          title: t("settings.providers.models_synced_title"),
          message: t("settings.providers.models_synced_message", {
            count: result.count,
          }),
        };
      } catch (error) {
        outcome = {
          tone: "error",
          title: t("settings.providers.models_sync_failed_title"),
          message: getProviderErrorMessage(
            error,
            t("settings.providers.models_sync_failed_message"),
          ),
        };
      } finally {
        if (targetProvider) {
          await refreshAll(targetProvider);
        }
        if (outcome) {
          setFeedback(outcome);
        }
      }
    });
  }, [
    modelApi,
    persistProvider,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
  ]);

  const handleAddModel = useCallback(() => {
    if (!selectedRecord || !selectedCanManage) {
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
    void runCommand({ kind: "add-model", modelId }, async () => {
      try {
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
          message: getProviderErrorMessage(
            error,
            t("settings.providers.model_add_failed_message"),
          ),
        });
      }
    });
  }, [
    manualModelEnabled,
    manualModelId,
    modelApi,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setAddModelOpen,
    setFeedback,
    setManualModelId,
    t,
  ]);

  const handleToggleModel = useCallback((
    model: ProviderModelRecord,
    enabled: boolean,
  ) => {
    if (!selectedRecord || !selectedCanManage) {
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
    void runCommand({ kind: "toggle-model", modelId: model.model_id }, async () => {
      try {
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
          message: getProviderErrorMessage(
            error,
            t("settings.providers.retry_later"),
          ),
        });
      }
    });
  }, [
    modelApi,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
  ]);

  const handleSaveModelOptions = useCallback(() => {
    if (!selectedRecord || !modelOptions || !selectedCanManage) {
      return;
    }
    void runCommand({
      kind: "save-model-options",
      modelId: modelOptions.model.model_id,
    }, async () => {
      try {
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
          message: getProviderErrorMessage(
            error,
            t("settings.providers.check_json_format"),
          ),
        });
      }
    });
  }, [
    modelApi,
    modelOptions,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    setModelOptions,
    t,
  ]);

  return {
    handleAddModel,
    handleFetchModels,
    handleSaveModelOptions,
    handleToggleModel,
  };
}
