import { useCallback, useMemo, useState } from "react";

import type {
  SkillMarketplaceFeedback,
  SkillMarketplaceFeedbackActions,
  SkillMarketplaceFeedbackInput,
  SkillMarketplaceFeedbackTone,
} from "./skill-marketplace-controller";

type FeedbackState = SkillMarketplaceFeedbackInput;

export function useSkillMarketplaceFeedback(): {
  actions: SkillMarketplaceFeedbackActions;
  feedback: SkillMarketplaceFeedback | null;
} {
  const [state, setState] = useState<FeedbackState | null>(null);
  const clear = useCallback(() => setState(null), []);
  const publish = useCallback(
    (tone: SkillMarketplaceFeedbackTone, message: string, pending = false) => {
      setState({ message, pending, tone });
    },
    [],
  );
  const report = useCallback((next: SkillMarketplaceFeedbackInput) => {
    setState(next);
  }, []);
  const actions = useMemo<SkillMarketplaceFeedbackActions>(
    () => ({
      clear,
      error: (message) => publish("error", message),
      report,
      start: (message) => publish("warning", message, true),
      success: (message) => publish("success", message),
      warning: (message) => publish("warning", message),
    }),
    [clear, publish, report],
  );
  const feedback = useMemo<SkillMarketplaceFeedback | null>(
    () => state ? { ...state, dismiss: clear } : null,
    [clear, state],
  );
  return { actions, feedback };
}
