import { useCallback, useEffect, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import type { ProviderModelApi } from "../../provider-settings-api";
import {
  buildProviderCommittedRefreshFeedback,
  buildProviderErrorFeedback,
  buildProviderValidationFeedback,
} from "../../model/provider-feedback-model";
import {
  isDefaultModelDisable,
  modelUpdatePayload,
  parseProviderOptions,
} from "../../model/provider-model-model";
import type {
  FeedbackState,
  ModelOptionsState,
} from "../../model/provider-settings-types";
import type { RunProviderCommand } from "../use-provider-command";

interface UseProviderModelUpdateOptions {
  modelApi: Pick<ProviderModelApi, "deleteModel" | "updateModel">;
  modelOptions: ModelOptionsState | null;
  refreshAll: (preferredProvider?: string | null) => Promise<boolean>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  setModelOptions: Dispatch<SetStateAction<ModelOptionsState | null>>;
  t: I18nContextValue["t"];
}

export function useProviderModelUpdate({
  modelApi,
  modelOptions,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  setModelOptions,
  t,
}: UseProviderModelUpdateOptions) {
  const [deleteModelTarget, setDeleteModelTarget] =
    useState<ProviderModelRecord | null>(null);

  useEffect(() => {
    setDeleteModelTarget(null);
  }, [selectedRecord?.provider]);

  const handleDefaultModelDisableAttempt = useCallback((
    model: ProviderModelRecord,
  ) => {
    setFeedback(buildProviderValidationFeedback(
      t("settings.providers.default_model_disable_title"),
      t("settings.providers.default_model_disable_message", {
        model: model.display_name || model.model_id,
      }),
      t,
    ));
  }, [setFeedback, t]);

  const handleToggleModel = useCallback((
    model: ProviderModelRecord,
    enabled: boolean,
  ) => {
    if (!selectedRecord || !selectedCanManage) {
      return;
    }
    if (isDefaultModelDisable(model, enabled)) {
      handleDefaultModelDisableAttempt(model);
      return;
    }
    void runCommand({ kind: "toggle-model", modelId: model.model_id }, async () => {
      try {
        await modelApi.updateModel(
          selectedRecord.provider,
          model.model_id,
          modelUpdatePayload(model, { enabled }),
        );
        if (!await refreshAll(selectedRecord.provider)) {
          setFeedback(buildProviderCommittedRefreshFeedback(
            t("settings.providers.refresh_after_change_failed_message"),
            t,
          ));
        }
      } catch (error) {
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.model_status_failed_title"),
          t("settings.providers.retry_later"),
          t,
        ));
      }
    });
  }, [
    handleDefaultModelDisableAttempt,
    modelApi,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
  ]);

  const handleRequestDeleteModel = useCallback((model: ProviderModelRecord) => {
    if (!selectedCanManage) {
      return;
    }
    if (model.is_default) {
      handleDefaultModelDisableAttempt(model);
      return;
    }
    setDeleteModelTarget(model);
  }, [handleDefaultModelDisableAttempt, selectedCanManage]);

  const handleDeleteModel = useCallback(() => {
    if (!selectedRecord || !deleteModelTarget || !selectedCanManage) {
      return;
    }
    void runCommand({
      kind: "delete-model",
      modelId: deleteModelTarget.model_id,
    }, async () => {
      try {
        await modelApi.deleteModel(
          selectedRecord.provider,
          deleteModelTarget.model_id,
        );
        setDeleteModelTarget(null);
        await refreshAll(selectedRecord.provider);
        setFeedback({
          tone: "success",
          title: t("settings.providers.model_deleted_title"),
          message: t("settings.providers.model_deleted_message", {
            model: deleteModelTarget.display_name || deleteModelTarget.model_id,
          }),
        });
      } catch (error) {
        setDeleteModelTarget(null);
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.model_delete_failed_title"),
          t("settings.providers.retry_later"),
          t,
        ));
      }
    });
  }, [
    deleteModelTarget,
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
    let providerOptions: Record<string, unknown> | undefined;
    try {
      providerOptions = parseProviderOptions(
        modelOptions.provider_options_text,
        t("settings.providers.provider_options_json_object"),
      );
    } catch {
      setFeedback(buildProviderValidationFeedback(
        t("settings.providers.model_options_save_failed_title"),
        t("settings.providers.check_json_format"),
        t,
      ));
      return;
    }
    void runCommand({
      kind: "save-model-options",
      modelId: modelOptions.model.model_id,
    }, async () => {
      try {
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
        if (!await refreshAll(selectedRecord.provider)) {
          setFeedback(buildProviderCommittedRefreshFeedback(
            t("settings.providers.refresh_after_change_failed_message"),
            t,
          ));
        }
      } catch (error) {
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.model_options_save_failed_title"),
          t("settings.providers.retry_later"),
          t,
        ));
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
    closeDeleteModelDialog: () => setDeleteModelTarget(null),
    deleteModelTarget,
    handleDefaultModelDisableAttempt,
    handleDeleteModel,
    handleRequestDeleteModel,
    handleSaveModelOptions,
    handleToggleModel,
  };
}
