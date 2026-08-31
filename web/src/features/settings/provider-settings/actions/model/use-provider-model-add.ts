import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import type { ProviderModelApi } from "../../provider-settings-api";
import {
  buildProviderCommittedRefreshFeedback,
  buildProviderErrorFeedback,
  buildProviderValidationFeedback,
} from "../../model/provider-feedback-model";
import { buildNewModelPayload } from "../../model/provider-model-model";
import type { FeedbackState } from "../../model/provider-settings-types";
import type { RunProviderCommand } from "../use-provider-command";

interface UseProviderModelAddOptions {
  manualModelEnabled: boolean;
  manualModelId: string;
  modelApi: Pick<ProviderModelApi, "updateModel">;
  refreshAll: (preferredProvider?: string | null) => Promise<boolean>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setAddModelOpen: Dispatch<SetStateAction<boolean>>;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  setManualModelId: Dispatch<SetStateAction<string>>;
  t: I18nContextValue["t"];
}

export function useProviderModelAdd({
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
}: UseProviderModelAddOptions) {
  const handleAddModel = useCallback(() => {
    if (!selectedRecord || !selectedCanManage) {
      return;
    }
    const modelId = manualModelId.trim();
    if (!modelId) {
      setFeedback(buildProviderValidationFeedback(
        t("settings.providers.model_id_required_title"),
        t("settings.providers.model_id_required_message"),
        t,
      ));
      return;
    }
    void runCommand({ kind: "add-model", modelId }, async () => {
      try {
        await modelApi.updateModel(
          selectedRecord.provider,
          modelId,
          buildNewModelPayload(manualModelEnabled),
        );
        setAddModelOpen(false);
        setManualModelId("");
        if (!await refreshAll(selectedRecord.provider)) {
          setFeedback(buildProviderCommittedRefreshFeedback(
            t("settings.providers.refresh_after_change_failed_message"),
            t,
          ));
          return;
        }
        setFeedback({
          tone: "success",
          title: t("settings.providers.model_added_title"),
          message: t("settings.providers.model_added_message", {
            model: modelId,
          }),
        });
      } catch (error) {
        setFeedback(buildProviderErrorFeedback(
          error,
          t("settings.providers.model_add_failed_title"),
          t("settings.providers.model_add_failed_message"),
          t,
        ));
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

  return { handleAddModel };
}
