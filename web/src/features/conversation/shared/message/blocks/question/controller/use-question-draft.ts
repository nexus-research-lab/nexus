import { useCallback, useEffect, useMemo } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message/content";

import { buildSubmittedQuestionDraft } from "../ask-user-question-model";
import {
  resolveInitialQuestionDraft,
  shouldRestoreSubmittedDraft,
  updateQuestionDraftCustomAnswer,
  updateQuestionDraftOption,
} from "./question-controller-model";

interface UseQuestionDraftOptions {
  initialSubmitted: boolean;
  questions: UserQuestion[];
  readOnly: boolean;
  scopeKey: string;
  toolResult?: ToolResultContent;
}

export function useQuestionDraft({
  initialSubmitted,
  questions,
  readOnly,
  scopeKey,
  toolResult,
}: UseQuestionDraftOptions) {
  const submittedDraft = useMemo(
    () => buildSubmittedQuestionDraft(questions, toolResult),
    [questions, toolResult],
  );
  const initialDraft = useMemo(
    () => resolveInitialQuestionDraft(
      questions,
      submittedDraft,
      initialSubmitted,
    ),
    [initialSubmitted, questions, submittedDraft],
  );
  const [draft, setDraft] = useResettableState(initialDraft, scopeKey);
  const restoreSubmittedDraft = shouldRestoreSubmittedDraft(
    initialSubmitted,
    submittedDraft,
  );

  useEffect(() => {
    if (restoreSubmittedDraft) {
      setDraft(submittedDraft);
    }
  }, [restoreSubmittedDraft, setDraft, submittedDraft]);

  const toggleOption = useCallback((
    questionIndex: number,
    optionLabel: string,
  ) => {
    setDraft((current) => updateQuestionDraftOption({
      draft: current,
      optionLabel,
      questionIndex,
      questions,
      readOnly,
    }));
  }, [questions, readOnly, setDraft]);

  const updateCustomAnswer = useCallback((
    questionIndex: number,
    customAnswer: string,
  ) => {
    setDraft((current) => updateQuestionDraftCustomAnswer({
      customAnswer,
      draft: current,
      questionIndex,
      questions,
      readOnly,
    }));
  }, [questions, readOnly, setDraft]);

  return { draft, toggleOption, updateCustomAnswer };
}

export function useQuestionExpansion(
  scopeKey: string,
  terminal: boolean,
) {
  const [expanded, setExpanded] = useResettableState(!terminal, scopeKey);
  useEffect(() => {
    if (terminal) {
      setExpanded(false);
    }
  }, [setExpanded, terminal]);
  return { expanded, setExpanded };
}
