import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { FeedbackState } from "../../model/provider-settings-types";
import type { PersistProvider } from "../config/use-provider-persistence";
import type {
  ProviderPendingAction,
  RunProviderCommand,
} from "../use-provider-command";

interface UseProviderPersistedModelCommandOptions {
  persistProvider: PersistProvider;
  refreshAll: (preferredProvider?: string | null) => Promise<void>;
  runCommand: RunProviderCommand;
  setFeedback: Dispatch<SetStateAction<FeedbackState | null>>;
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
}: UseProviderPersistedModelCommandOptions): RunPersistedModelCommand {
  return useCallback((action, request, buildFailure) => {
    void runCommand(action, async () => {
      let targetProvider: string | null = null;
      let outcome: FeedbackState | null = null;
      try {
        const persisted = await persistProvider({ showError: true });
        if (!persisted) {
          return;
        }
        targetProvider = persisted.record.provider;
        outcome = await request(targetProvider);
      } catch (error) {
        outcome = buildFailure(error);
      } finally {
        if (targetProvider) {
          await refreshAll(targetProvider);
        }
        if (outcome) {
          setFeedback(outcome);
        }
      }
    });
  }, [persistProvider, refreshAll, runCommand, setFeedback]);
}
