import { useCallback, useMemo } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type {
  UserQuestion,
  UserQuestionAnswer,
} from "@/types/conversation/interaction/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message/content";

import { createQuestionScopeKey } from "../ask-user-question-model";
import {
  isQuestionSubmissionReady,
  projectQuestionDraftMetrics,
  projectQuestionInteraction,
  shouldAutoSubmitQuestionCard,
  updateQuestionDraftCustomAnswer,
  updateQuestionDraftOption,
} from "./question-controller-model";
import {
  useQuestionDraft,
  useQuestionExpansion,
} from "./use-question-draft";
import { useQuestionSubmission } from "./use-question-submission";

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
  const [hasLocalSubmission, setHasLocalSubmission] = useResettableState(
    false,
    scopeKey,
  );
  const interaction = projectQuestionInteraction({
    hasLocalSubmission,
    initialSubmitted,
    interactionDisabled,
    toolResult,
  });
  const {
    draft,
    replaceDraft,
    updateCustomAnswer,
  } = useQuestionDraft({
    initialSubmitted,
    questions,
    readOnly: interaction.readOnly,
    scopeKey,
    toolResult,
  });
  const { expanded, setExpanded } = useQuestionExpansion(
    scopeKey,
    interaction.terminal,
  );
  const metrics = useMemo(
    () => projectQuestionDraftMetrics(
      questions,
      draft,
      interaction.submitted,
    ),
    [draft, interaction.submitted, questions],
  );
  const submissionReady = isQuestionSubmissionReady({
    draftComplete: metrics.complete,
    isReady,
    readOnly: interaction.readOnly,
  });
  const autoSubmit = shouldAutoSubmitQuestionCard(questions);
  const handleAccepted = useCallback(
    () => setHasLocalSubmission(true),
    [setHasLocalSubmission],
  );
  const handleCollapse = useCallback(
    () => setExpanded(false),
    [setExpanded],
  );
  const submission = useQuestionSubmission({
    draft,
    onAccepted: handleAccepted,
    onCollapse: handleCollapse,
    onSubmit,
    scopeKey,
    submissionReady,
    toolUseId,
  });
  const submitQuestion = submission.submit;
  const submitUpdatedDraft = useCallback((nextDraft: typeof draft) => {
    replaceDraft(nextDraft);
    const nextMetrics = projectQuestionDraftMetrics(
      questions,
      nextDraft,
      false,
    );
    const nextSubmissionReady = isQuestionSubmissionReady({
      draftComplete: nextMetrics.complete,
      isReady,
      readOnly: interaction.readOnly,
    });
    void submitQuestion(nextDraft, nextSubmissionReady);
  }, [
    interaction.readOnly,
    isReady,
    questions,
    replaceDraft,
    submitQuestion,
  ]);
  const handleToggleOption = useCallback((
    questionIndex: number,
    optionLabel: string,
  ) => {
    const nextDraft = updateQuestionDraftOption({
      draft,
      optionLabel,
      questionIndex,
      questions,
      readOnly: interaction.readOnly,
    });
    if (autoSubmit) {
      submitUpdatedDraft(nextDraft);
      return;
    }
    replaceDraft(nextDraft);
  }, [
    autoSubmit,
    draft,
    interaction.readOnly,
    questions,
    replaceDraft,
    submitUpdatedDraft,
  ]);
  const handleCustomAnswerSubmit = useCallback((
    questionIndex: number,
    customAnswer: string,
  ) => {
    if (!autoSubmit || !customAnswer.trim()) {
      return;
    }
    const nextDraft = updateQuestionDraftCustomAnswer({
      customAnswer,
      draft,
      questionIndex,
      questions,
      readOnly: interaction.readOnly,
    });
    submitUpdatedDraft(nextDraft);
  }, [
    autoSubmit,
    draft,
    interaction.readOnly,
    questions,
    submitUpdatedDraft,
  ]);

  return {
    answerSummary: metrics.answerSummary,
    autoSubmit,
    draft,
    draftComplete: metrics.complete,
    isExpanded: expanded,
    isReady,
    isSubmitting: submission.isSubmitting,
    readOnly: interaction.readOnly,
    setIsExpanded: setExpanded,
    status: interaction.status,
    submit: submission.submit,
    submitCustomAnswer: handleCustomAnswerSubmit,
    submitEnabled: submission.submitEnabled,
    toggleOption: handleToggleOption,
    totalSelected: metrics.totalSelected,
    updateCustomAnswer,
  };
}
