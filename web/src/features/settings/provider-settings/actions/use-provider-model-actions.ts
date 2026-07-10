import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
} from "@/types/capability/provider";

import type { ProviderModelApi } from "../provider-settings-api";
import type { FeedbackState } from "../model/provider-settings-types";
import type { PersistProvider } from "./use-provider-config-actions";
import type { RunProviderCommand } from "./use-provider-command";
import { useProviderModelControls } from "./use-provider-model-controls";
import { useProviderModelMutations } from "./use-provider-model-mutations";
import { useProviderTestActions } from "./use-provider-test-actions";

interface UseProviderModelActionsOptions {
  apiFormat: ProviderApiFormat;
  modelApi: ProviderModelApi;
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
}

export function useProviderModelActions({
  apiFormat,
  modelApi,
  persistProvider,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  t,
}: UseProviderModelActionsOptions) {
  const controls = useProviderModelControls({
    apiFormat,
    selectedCanManage,
    selectedRecord,
    t,
  });
  const mutations = useProviderModelMutations({
    manualModelEnabled: controls.manualModelEnabled,
    manualModelId: controls.manualModelId,
    modelApi,
    modelOptions: controls.modelOptions,
    persistProvider,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setAddModelOpen: controls.setAddModelOpen,
    setFeedback,
    setManualModelId: controls.setManualModelId,
    setModelOptions: controls.setModelOptions,
    t,
  });
  const tests = useProviderTestActions({
    modelApi,
    persistProvider,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
  });

  return { ...controls, ...mutations, ...tests };
}
