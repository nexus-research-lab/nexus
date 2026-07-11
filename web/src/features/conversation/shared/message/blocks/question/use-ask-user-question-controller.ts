import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  type UserQuestion,
  type UserQuestionAnswer,
} from "@/types/conversation/interaction/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message/content";

import {
  buildQuestionAnswers,
  buildSubmittedQuestionDraft,
  countQuestionAnswers,
  createEmptyQuestionDraft,
  createQuestionScopeKey,
  hasQuestionDraftContent,
  isQuestionDraftComplete,
  isQuestionStatusTerminal,
  resolveQuestionInteractionStatus,
  summarizeQuestionAnswers,
  toggleQuestionOption,
  updateQuestionCustomAnswer,
} from "./ask-user-question-model";
import { isAskUserQuestionTimedOutResult } from "./ask-user-question-timeout";

interface UseAskUserQuestionControllerParams {
  initialSubmitted: boolean;
  interactionDisabled: boolean;
  isReady: boolean;
  onSubmit: (
    toolUseId: string,
    answers: UserQuestionAnswer[],
  ) => boolean | Promise<boolean>;
  questions: UserQuestion[];
  toolResult?: ToolResultContent;
  toolUseId: string;
}

interface SubmissionToken {
  scopeKey: string;
}

export function useAskUserQuestionController({
  initialSubmitted,
  interactionDisabled,
  isReady,
  onSubmit,
  questions,
  toolResult,
  toolUseId,
}: UseAskUserQuestionControllerParams) {
  const scopeKey = useMemo(
    () => createQuestionScopeKey(toolUseId, questions),
    [questions, toolUseId],
  );
  const activeScopeRef = useRef(scopeKey);
  activeScopeRef.current = scopeKey;
  const activeSubmissionRef = useRef<SubmissionToken | null>(null);
  const submittedDraft = useMemo(
    () => buildSubmittedQuestionDraft(questions, toolResult),
    [questions, toolResult],
  );
  const submittedDraftHasContent = useMemo(
    () => hasQuestionDraftContent(submittedDraft),
    [submittedDraft],
  );
  const initialDraft = initialSubmitted || submittedDraftHasContent
    ? submittedDraft
    : createEmptyQuestionDraft(questions.length);
  const [draft, setDraft] = useResettableState(
    initialDraft,
    scopeKey,
  );
  const [hasLocalSubmission, setHasLocalSubmission] = useResettableState(
    false,
    scopeKey,
  );
  const [isSubmitting, setIsSubmitting] = useResettableState(false, scopeKey);
  const submitted = initialSubmitted || hasLocalSubmission;
  const timedOut = isAskUserQuestionTimedOutResult(toolResult);
  const failed = Boolean(toolResult?.is_error && !timedOut);
  const status = resolveQuestionInteractionStatus({
    failed,
    interactionDisabled,
    submitted,
    timedOut,
  });
  const readOnly = status !== "active";
  const terminal = isQuestionStatusTerminal(status);
  const [isExpanded, setIsExpanded] = useResettableState(
    !terminal,
    scopeKey,
  );

  useEffect(() => () => {
    if (activeScopeRef.current === scopeKey) {
      activeScopeRef.current = "";
    }
  }, [scopeKey]);

  useEffect(() => {
    if (terminal) {
      setIsExpanded(false);
    }
  }, [setIsExpanded, terminal]);

  useEffect(() => {
    if (!initialSubmitted && !submittedDraftHasContent) {
      return;
    }
    setDraft(submittedDraft);
  }, [initialSubmitted, setDraft, submittedDraft, submittedDraftHasContent]);

  const toggleOption = useCallback((
    questionIndex: number,
    optionLabel: string,
  ): void => {
    if (readOnly) {
      return;
    }
    const question = questions[questionIndex];
    if (!question) {
      return;
    }
    setDraft((current) => toggleQuestionOption(
      current,
      questionIndex,
      optionLabel,
      Boolean(question.multi_select),
    ));
  }, [questions, readOnly, setDraft]);

  const updateCustomAnswer = useCallback((
    questionIndex: number,
    customAnswer: string,
  ): void => {
    if (readOnly) {
      return;
    }
    const question = questions[questionIndex];
    if (!question) {
      return;
    }
    setDraft((current) => updateQuestionCustomAnswer(
      current,
      questionIndex,
      customAnswer,
      Boolean(question.multi_select),
    ));
  }, [questions, readOnly, setDraft]);

  const draftComplete = useMemo(
    () => isQuestionDraftComplete(questions, draft),
    [draft, questions],
  );
  const totalSelected = useMemo(() => countQuestionAnswers(draft), [draft]);
  const answerSummary = useMemo(
    () => submitted ? summarizeQuestionAnswers(draft) : "",
    [draft, submitted],
  );
  const submitEnabled = draftComplete
    && isReady
    && !readOnly
    && !isSubmitting;

  const submit = useCallback(async (): Promise<void> => {
    if (!submitEnabled || activeSubmissionRef.current?.scopeKey === scopeKey) {
      return;
    }

    const token = { scopeKey };
    activeSubmissionRef.current = token;
    setIsSubmitting(true);
    try {
      const accepted = await onSubmit(toolUseId, buildQuestionAnswers(draft));
      if (!accepted || activeScopeRef.current !== token.scopeKey) {
        return;
      }
      setHasLocalSubmission(true);
      setIsExpanded(false);
    } finally {
      if (activeSubmissionRef.current === token) {
        activeSubmissionRef.current = null;
      }
      if (activeScopeRef.current === token.scopeKey) {
        setIsSubmitting(false);
      }
    }
  }, [
    draft,
    onSubmit,
    scopeKey,
    setHasLocalSubmission,
    setIsExpanded,
    setIsSubmitting,
    submitEnabled,
    toolUseId,
  ]);

  return {
    answerSummary,
    draft,
    draftComplete,
    isExpanded,
    isReady,
    isSubmitting,
    readOnly,
    setIsExpanded,
    status,
    submit,
    submitEnabled,
    toggleOption,
    totalSelected,
    updateCustomAnswer,
  };
}
