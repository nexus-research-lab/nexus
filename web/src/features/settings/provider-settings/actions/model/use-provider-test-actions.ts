// INPUT: Explicit Provider/model test and independent capability outcomes.
// OUTPUT: Existing feedback and refresh flow with unknown/unsupported distinctions.
// POS: Provider test action; automatic observations never become manual overrides.
import { useCallback, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderTestResult,
} from "@/types/capability/provider";

import type { ProviderModelApi } from "../../provider-settings-api";
import { buildProviderErrorFeedback } from "../../model/provider-feedback-model";
import { runCapabilityBatch } from "./provider-capability-batch";
import { AUTO_TEST_MODEL_VALUE } from "../../model/provider-model-model";
import type { FeedbackState } from "../../model/provider-settings-types";
import type { PersistProvider } from "../config/use-provider-persistence";
import type {
  ProviderPendingAction,
  RunProviderCommand,
} from "../use-provider-command";
import { useProviderPersistedModelCommand } from "./use-provider-persisted-model-command";

interface UseProviderTestActionsOptions {
  modelApi: Pick<ProviderModelApi, "testModel">;
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<boolean>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
}

interface TestMessages {
  failureImpact: string;
  failureFallback: string;
  failureTitle: string;
  successFallbackModel: string;
  successTitle: string;
  incompleteTitle: string;
  incompleteImpact: string;
  deniedTitle: string;
  deniedImpact: string;
}

function buildTestFeedback(
  result: ProviderTestResult,
  messages: TestMessages,
  formatSuccess: (model: string) => string,
): FeedbackState {
  if (result.capability_results && Object.values(result.capability_results).some((check) => check?.state === "unknown" || check?.state === "error")) {
    return { tone: "warning", title: messages.incompleteTitle, impact: messages.incompleteImpact };
  }
  if (result.capability_results && Object.values(result.capability_results).some((check) => check?.state === "unsupported")) {
    return { tone: "warning", title: messages.deniedTitle, impact: messages.deniedImpact };
  }
  if (result.success) {
    return {
      message: formatSuccess(result.model || messages.successFallbackModel),
      title: messages.successTitle,
      tone: "success",
    };
  }
  return {
    impact: messages.failureImpact,
    tone: "error",
    title: messages.failureTitle,
  };
}

export function useProviderTestActions({
  modelApi,
  persistProvider,
  refreshAll,
  runCommand,
  selectedCanManage,
  selectedRecord,
  setFeedback,
  t,
}: UseProviderTestActionsOptions) {
  const stopTests = useRef(false);
  const [testProgress, setTestProgress] = useState<{ done: number; total: number } | null>(null);
  const handleStopTests = useCallback(() => { stopTests.current = true; }, []);
  const runPersistedModelCommand = useProviderPersistedModelCommand({
    persistProvider,
    refreshAll,
    runCommand,
    setFeedback,
    t,
  });

  const runTest = useCallback((
    action: ProviderPendingAction,
    request: (provider: string) => Promise<ProviderTestResult>,
    messages: TestMessages,
  ) => {
    if (!selectedRecord || !selectedCanManage) {
      return;
    }
    runPersistedModelCommand(
      action,
      async (provider) => buildTestFeedback(
        await request(provider),
        messages,
        (model) => t("settings.providers.test_model_message", { model }),
      ),
      (error) => buildProviderErrorFeedback(
        error,
        messages.failureTitle,
        messages.failureFallback,
        t,
      ),
    );
  }, [
    runPersistedModelCommand,
    selectedCanManage,
    selectedRecord,
    t,
  ]);

  const handleTestProvider = useCallback(() => {
    if (!selectedRecord || !selectedCanManage) return;
    runPersistedModelCommand({ kind: "test-provider" }, async (provider) => {
      stopTests.current = false;
      try {
        const batch = await runCapabilityBatch({
          models: selectedRecord.models.map((model) => model.model_id),
          test: (model) => modelApi.testModel(provider, model, { capability: "all" }),
          shouldStop: () => stopTests.current,
          onProgress: (done, total) => setTestProgress({ done, total }),
        });
        if (batch.stopped) return { tone: "warning", title: t("settings.providers.tests_stopped"), impact: t("settings.providers.tests_stopped_impact") };
        const incomplete = batch.results.some((result) => !result.success || Object.values(result.capability_results ?? {}).some((check) => check?.state === "unknown" || check?.state === "error"));
        const denied = batch.results.some((result) => Object.values(result.capability_results ?? {}).some((check) => check?.state === "unsupported"));
        if (incomplete || denied) return {
          tone: "warning",
          title: t(incomplete ? "settings.providers.capability_test_incomplete" : "settings.providers.capability_test_denied"),
          impact: t(incomplete ? "settings.providers.capability_test_incomplete_impact" : "settings.providers.capability_test_denied_impact"),
        };
        return { tone: "success", title: t("settings.providers.provider_test_passed_title"), message: t("settings.providers.all_models_tested", { count: batch.results.length }) };
      } finally {
        setTestProgress(null);
      }
    }, (error) => buildProviderErrorFeedback(error, t("settings.providers.provider_test_failed_title"), t("settings.providers.check_network_auth"), t));
  }, [modelApi, runPersistedModelCommand, selectedRecord, selectedCanManage, t]);

  const handleTestModel = useCallback((modelId: string) => {
    const normalizedModelId = modelId.trim();
    if (!normalizedModelId) {
      return;
    }
    runTest(
      { kind: "test-model", modelId: normalizedModelId },
      (provider) => modelApi.testModel(provider, normalizedModelId),
      {
        failureImpact: t("settings.providers.test_failed_impact"),
        incompleteTitle: t("settings.providers.capability_test_incomplete"),
        deniedTitle: t("settings.providers.capability_test_denied"),
        deniedImpact: t("settings.providers.capability_test_denied_impact"),
        incompleteImpact: t("settings.providers.capability_test_incomplete_impact"),
        failureFallback: t("settings.providers.check_network_auth_model"),
        failureTitle: t("settings.providers.model_test_failed_title"),
        successFallbackModel: normalizedModelId,
        successTitle: t("settings.providers.model_test_passed_title"),
      },
    );
  }, [modelApi, runTest, t]);

  const handleTestSelection = useCallback((value: string) => {
    if (value === AUTO_TEST_MODEL_VALUE) {
      handleTestProvider();
      return;
    }
    handleTestModel(value);
  }, [handleTestModel, handleTestProvider]);

  return { handleTestSelection, testProgress, handleStopTests };
}
