import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import { getErrorMessage } from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { ProviderConfigRecord } from "@/types/capability/provider";

import type { ProviderModelApi } from "../../provider-settings-api";
import type { FeedbackState } from "../../model/provider-settings-types";
import type { PersistProvider } from "../config/use-provider-persistence";
import type { RunProviderCommand } from "../use-provider-command";
import { useProviderPersistedModelCommand } from "./use-provider-persisted-model-command";

interface UseProviderModelSyncOptions {
  modelApi: Pick<ProviderModelApi, "fetchModels">;
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
}

export function useProviderModelSync({
  modelApi,
  persistProvider,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  t,
}: UseProviderModelSyncOptions) {
  const runPersistedModelCommand = useProviderPersistedModelCommand({
    persistProvider,
    refreshAll,
    runCommand,
    setFeedback,
  });

  const handleFetchModels = useCallback(() => {
    if (!selectedRecord || !selectedCanManage) {
      return;
    }
    runPersistedModelCommand(
      { kind: "fetch-models" },
      async (provider) => {
        const result = await modelApi.fetchModels(provider);
        return {
          tone: "success",
          title: t("settings.providers.models_synced_title"),
          message: t("settings.providers.models_synced_message", {
            count: result.count,
          }),
        };
      },
      (error) => ({
        tone: "error",
        title: t("settings.providers.models_sync_failed_title"),
        message: getErrorMessage(
          error,
          t("settings.providers.models_sync_failed_message"),
        ),
      }),
    );
  }, [
    modelApi,
    runPersistedModelCommand,
    selectedCanManage,
    selectedRecord,
    t,
  ]);

  return { handleFetchModels };
}
