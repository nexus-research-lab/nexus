import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ProviderConfigRecord,
  ProviderTestResult,
} from "@/types/capability/provider";

import type { ProviderModelApi } from "../provider-settings-api";
import { getProviderErrorMessage } from "../model/provider-feedback-model";
import { AUTO_TEST_MODEL_VALUE } from "../model/provider-model-model";
import type { FeedbackState } from "../model/provider-settings-types";
import type { PersistProvider } from "./config/use-provider-persistence";
import type {
  ProviderPendingAction,
  RunProviderCommand,
} from "./use-provider-command";

interface UseProviderTestActionsOptions {
  modelApi: ProviderModelApi;
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  runCommand: RunProviderCommand;
  selectedCanManage: boolean;
  selectedRecord: ProviderConfigRecord | null;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
}

interface TestMessages {
  failureFallback: string;
  failureTitle: string;
  successFallbackModel: string;
  successTitle: string;
}

function buildTestFeedback(
  result: ProviderTestResult,
  messages: TestMessages,
  formatSuccess: (model: string) => string,
): FeedbackState {
  return {
    tone: result.success ? "success" : "error",
    title: result.success ? messages.successTitle : messages.failureTitle,
    message: result.success
      ? formatSuccess(result.model || messages.successFallbackModel)
      : result.error || messages.failureFallback,
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
  const runTest = useCallback((
    action: ProviderPendingAction,
    request: (provider: string) => Promise<ProviderTestResult>,
    messages: TestMessages,
  ) => {
    if (!selectedRecord || !selectedCanManage) {
      return;
    }
    void runCommand(action, async () => {
      let targetProvider: string | null = null;
      let outcome: FeedbackState | null = null;
      try {
        const persisted = await persistProvider({ showError: true });
        if (!persisted) {
          return;
        }
        targetProvider = persisted.record.provider;
        const result = await request(targetProvider);
        outcome = buildTestFeedback(
          result,
          messages,
          (model) => t("settings.providers.test_model_message", { model }),
        );
      } catch (error) {
        outcome = {
          tone: "error",
          title: messages.failureTitle,
          message: getProviderErrorMessage(error, messages.failureFallback),
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
    persistProvider,
    refreshAll,
    runCommand,
    selectedCanManage,
    selectedRecord,
    setFeedback,
    t,
  ]);

  const handleTestProvider = useCallback(() => {
    runTest(
      { kind: "test-provider" },
      (provider) => modelApi.testProvider(provider),
      {
        failureFallback: t("settings.providers.check_network_auth"),
        failureTitle: t("settings.providers.provider_test_failed_title"),
        successFallbackModel: t("settings.providers.auto_model"),
        successTitle: t("settings.providers.provider_test_passed_title"),
      },
    );
  }, [modelApi, runTest, t]);

  const handleTestModel = useCallback((modelId: string) => {
    const normalizedModelId = modelId.trim();
    if (!normalizedModelId) {
      return;
    }
    runTest(
      { kind: "test-model", modelId: normalizedModelId },
      (provider) => modelApi.testModel(provider, normalizedModelId),
      {
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

  return { handleTestSelection };
}
