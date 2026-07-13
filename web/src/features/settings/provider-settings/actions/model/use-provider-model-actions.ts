import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
} from "@/types/capability/provider";

import type { ProviderModelApi } from "../../provider-settings-api";
import type { FeedbackState } from "../../model/provider-settings-types";
import type { PersistProvider } from "../config/use-provider-persistence";
import type { RunProviderCommand } from "../use-provider-command";
import { useProviderModelAdd } from "./use-provider-model-add";
import { useProviderModelControls } from "./use-provider-model-controls";
import { useProviderModelSync } from "./use-provider-model-sync";
import { useProviderModelUpdate } from "./use-provider-model-update";
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
  const sync = useProviderModelSync({
    modelApi,
    persistProvider,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
  });
  const add = useProviderModelAdd({
    manualModelEnabled: controls.manualModelEnabled,
    manualModelId: controls.manualModelId,
    modelApi,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setAddModelOpen: controls.setAddModelOpen,
    setFeedback,
    setManualModelId: controls.setManualModelId,
    t,
  });
  const update = useProviderModelUpdate({
    modelApi,
    modelOptions: controls.modelOptions,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
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

  return { ...add, ...controls, ...sync, ...tests, ...update };
}
