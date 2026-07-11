import { useCallback, useEffect, useRef } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import type { SaveFeedback } from "../agent-options-editor-model";

export function useAgentSaveFeedback(scopeKey: string) {
  const [feedback, setFeedback] = useResettableState<SaveFeedback | null>(null, scopeKey);
  const timerRef = useRef<number | null>(null);

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const clear = useCallback(() => {
    clearTimer();
    setFeedback(null);
  }, [clearTimer, setFeedback]);

  useEffect(() => clearTimer, [clearTimer, scopeKey]);

  const showError = useCallback((message: string) => {
    clear();
    setFeedback({ message, tone: "error" });
  }, [clear, setFeedback]);

  const showSuccess = useCallback((message: string) => {
    clear();
    setFeedback({ message, tone: "success" });
    timerRef.current = window.setTimeout(() => {
      setFeedback((current) => current?.tone === "success" ? null : current);
      timerRef.current = null;
    }, 1_800);
  }, [clear, setFeedback]);

  return { clear, feedback, showError, showSuccess };
}
