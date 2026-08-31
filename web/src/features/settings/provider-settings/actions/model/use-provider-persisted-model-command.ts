import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import { buildProviderFollowupRefreshFailureFeedback } from "../../model/provider-feedback-model";
import type { FeedbackState } from "../../model/provider-settings-types";
import type { PersistProvider } from "../config/use-provider-persistence";
import type {
  ProviderPendingAction,
  RunProviderCommand,
} from "../use-provider-command";

interface UseProviderPersistedModelCommandOptions {
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<boolean>;
  runCommand: RunProviderCommand;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
  t: I18nContextValue["t"];
}

type RunPersistedModelCommand = (
  action: ProviderPendingAction,
  request: (provider: string) => Promise<FeedbackState>,
  buildFailure: (error: unknown) => FeedbackState,
) => void;

export function useProviderPersistedModelCommand({
  persistProvider,
  refreshAll,
  runCommand,
  setFeedback,
  t,
}: UseProviderPersistedModelCommandOptions): RunPersistedModelCommand {
  return useCallback((action, request, buildFailure) => {
    void runCommand(action, async () => {
      let targetProvider: string | null = null;
      let outcome: FeedbackState | null = null;
      let requestCompleted = false;
      try {
        const persisted = await persistProvider({ showError: true });
        if (!persisted) {
          return;
        }
        targetProvider = persisted.record.provider;
        outcome = await request(targetProvider);
        requestCompleted = true;
      } catch (error) {
        outcome = buildFailure(error);
      } finally {
        if (targetProvider) {
          const refreshed = await refreshAll(targetProvider);
          if (!refreshed && requestCompleted) {
            setFeedback(buildProviderFollowupRefreshFailureFeedback(t));
            return;
          }
        }
        if (outcome) {
          setFeedback(outcome);
        }
      }
    });
  }, [persistProvider, refreshAll, runCommand, setFeedback, t]);
}
