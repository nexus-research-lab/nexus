import type {
  UserQuestion,
} from "@/types/conversation/interaction/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message/content";

import {
  createEmptyQuestionDraft,
  hasQuestionDraftContent,
  isQuestionDraftComplete,
  isQuestionStatusTerminal,
  resolveQuestionInteractionStatus,
  summarizeQuestionAnswers,
  toggleQuestionOption,
  updateQuestionCustomAnswer,
  countQuestionAnswers,
  type QuestionDraft,
  type QuestionInteractionStatus,
} from "../ask-user-question-model";
import { isAskUserQuestionTimedOutResult } from "../ask-user-question-timeout";

export interface QuestionInteractionProjection {
  readOnly: boolean;
  status: QuestionInteractionStatus;
  submitted: boolean;
  terminal: boolean;
}

export interface QuestionDraftMetrics {
  answerSummary: string;
  complete: boolean;
  totalSelected: number;
}

export function projectQuestionInteraction({
  hasLocalSubmission,
  initialSubmitted,
  interactionDisabled,
  toolResult,
}: {
  hasLocalSubmission: boolean;
  initialSubmitted: boolean;
  interactionDisabled: boolean;
  toolResult?: ToolResultContent;
}): QuestionInteractionProjection {
  const submitted = [initialSubmitted, hasLocalSubmission].some(Boolean);
  const timedOut = isAskUserQuestionTimedOutResult(toolResult);
  const failed = [Boolean(toolResult?.is_error), !timedOut].every(Boolean);
  const status = resolveQuestionInteractionStatus({
    failed,
    interactionDisabled,
    submitted,
    timedOut,
  });
  return {
    readOnly: status !== "active",
    status,
    submitted,
    terminal: isQuestionStatusTerminal(status),
  };
}

export function resolveInitialQuestionDraft(
  questions: UserQuestion[],
  submittedDraft: QuestionDraft,
  initialSubmitted: boolean,
): QuestionDraft {
  const shouldRestore = shouldRestoreSubmittedDraft(
    initialSubmitted,
    submittedDraft,
  );
  return shouldRestore
    ? submittedDraft
    : createEmptyQuestionDraft(questions.length);
}

export function shouldRestoreSubmittedDraft(
  initialSubmitted: boolean,
  submittedDraft: QuestionDraft,
): boolean {
  return [initialSubmitted, hasQuestionDraftContent(submittedDraft)].some(
    Boolean,
  );
}

export function updateQuestionDraftOption({
  draft,
  optionLabel,
  questionIndex,
  questions,
  readOnly,
}: {
  draft: QuestionDraft;
  optionLabel: string;
  questionIndex: number;
  questions: UserQuestion[];
  readOnly: boolean;
}): QuestionDraft {
  const question = resolveEditableQuestion(questions, questionIndex, readOnly);
  if (!question) {
    return draft;
  }
  return toggleQuestionOption(
    draft,
    questionIndex,
    optionLabel,
    Boolean(question.multi_select),
  );
}

export function updateQuestionDraftCustomAnswer({
  customAnswer,
  draft,
  questionIndex,
  questions,
  readOnly,
}: {
  customAnswer: string;
  draft: QuestionDraft;
  questionIndex: number;
  questions: UserQuestion[];
  readOnly: boolean;
}): QuestionDraft {
  const question = resolveEditableQuestion(questions, questionIndex, readOnly);
  if (!question) {
    return draft;
  }
  return updateQuestionCustomAnswer(
    draft,
    questionIndex,
    customAnswer,
    Boolean(question.multi_select),
  );
}

function resolveEditableQuestion(
  questions: UserQuestion[],
  questionIndex: number,
  readOnly: boolean,
): UserQuestion | null {
  return readOnly ? null : questions[questionIndex] ?? null;
}

export function projectQuestionDraftMetrics(
  questions: UserQuestion[],
  draft: QuestionDraft,
  submitted: boolean,
): QuestionDraftMetrics {
  return {
    answerSummary: submitted ? summarizeQuestionAnswers(draft) : "",
    complete: isQuestionDraftComplete(questions, draft),
    totalSelected: countQuestionAnswers(draft),
  };
}

export function isQuestionSubmissionReady({
  draftComplete,
  isReady,
  readOnly,
}: {
  draftComplete: boolean;
  isReady: boolean;
  readOnly: boolean;
}): boolean {
  return [draftComplete, isReady, !readOnly].every(Boolean);
}
