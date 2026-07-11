import { useCallback, useMemo, useState } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderApiFormat,
  ProviderConfigRecord,
  ProviderModelRecord,
} from "@/types/capability/provider";

import {
  buildTestModelOptions,
  filterProviderModels,
  modelOptionsFromRecord,
  sortModelsEnabledFirst,
} from "../../model/provider-model-model";
import type { ModelOptionsState } from "../../model/provider-settings-types";

interface UseProviderModelControlsOptions {
  apiFormat: ProviderApiFormat;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  t: I18nContextValue["t"];
}

export function useProviderModelControls({
  apiFormat,
  selectedCanManage,
  selectedRecord,
  t,
}: UseProviderModelControlsOptions) {
  const [modelQuery, setModelQuery] = useState("");
  const [modelOptions, setModelOptions] =
    useState<ModelOptionsState | null>(null);
  const [addModelOpen, setAddModelOpen] = useState(false);
  const [manualModelId, setManualModelId] = useState("");
  const [manualModelEnabled, setManualModelEnabled] = useState(true);

  const displayedModels = useMemo(() => sortModelsEnabledFirst(
    filterProviderModels(selectedRecord?.models ?? [], modelQuery),
  ), [modelQuery, selectedRecord]);
  const testModelOptions = useMemo(() => buildTestModelOptions(
    selectedRecord?.models ?? [],
    t("settings.providers.auto_select_model"),
  ), [selectedRecord, t]);
  const manualModelPlaceholder = selectedRecord?.models[0]?.model_id
    || (apiFormat === "anthropic_messages" ? "opus-4.7" : "model-id");

  const resetModelControls = useCallback(() => {
    setModelQuery("");
    setAddModelOpen(false);
    setModelOptions(null);
    setManualModelId("");
    setManualModelEnabled(true);
  }, []);

  const handleOpenAddModel = useCallback(() => {
    if (!selectedCanManage) {
      return;
    }
    setManualModelId("");
    setManualModelEnabled(true);
    setAddModelOpen(true);
  }, [selectedCanManage]);

  const setModelOptionsFromRecord = useCallback((model: ProviderModelRecord) => {
    setModelOptions(modelOptionsFromRecord(model));
  }, []);

  return {
    addModelOpen,
    displayedModels,
    handleOpenAddModel,
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
    setModelOptionsFromRecord,
    setModelQuery,
    testModelOptions,
  };
}
