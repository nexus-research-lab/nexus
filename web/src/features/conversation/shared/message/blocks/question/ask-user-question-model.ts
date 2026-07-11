import type {
  UserQuestion,
  UserQuestionAnswer,
} from "@/types/conversation/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message";

interface QuestionAnswerDraft {
  customAnswer: string;
  selectedOptions: ReadonlySet<string>;
}

export type QuestionDraft = QuestionAnswerDraft[];

export type QuestionInteractionStatus =
  | "active"
  | "failed"
  | "observer"
  | "submitted"
  | "timed_out";

const TERMINAL_QUESTION_STATUSES = new Set<QuestionInteractionStatus>([
  "failed",
  "submitted",
  "timed_out",
]);

interface QuestionStatusInput {
  failed: boolean;
  interactionDisabled: boolean;
  submitted: boolean;
  timedOut: boolean;
}

export function normalizeQuestion(question: UserQuestion): UserQuestion {
  return {
    ...question,
    // SDK 可能直接透传 camelCase，视图内部只消费统一字段。
    multi_select: question.multi_select ?? question.multiSelect ?? false,
  };
}

export function createEmptyQuestionDraft(questionCount: number): QuestionDraft {
  return Array.from({ length: questionCount }, () => ({
    customAnswer: "",
    selectedOptions: new Set<string>(),
  }));
}

function extractAnswerPairs(content: string): Map<string, string> {
  const pairs = new Map<string, string>();
  const matcher = /"([^"]+)"="([^"]*)"/g;
  let match = matcher.exec(content);
  while (match) {
    pairs.set(match[1], match[2]);
    match = matcher.exec(content);
  }
  return pairs;
}

function restoreQuestionAnswer(
  question: UserQuestion,
  answerText: string,
): QuestionAnswerDraft {
  const optionLabels = new Set(question.options.map((option) => option.label));
  if (!question.multi_select) {
    return optionLabels.has(answerText)
      ? { customAnswer: "", selectedOptions: new Set([answerText]) }
      : { customAnswer: answerText, selectedOptions: new Set() };
  }

  const answerItems = answerText
    .split(", ")
    .map((item) => item.trim())
    .filter(Boolean);
  return {
    customAnswer: answerItems
      .filter((item) => !optionLabels.has(item))
      .join(", "),
    selectedOptions: new Set(
      answerItems.filter((item) => optionLabels.has(item)),
    ),
  };
}

export function buildSubmittedQuestionDraft(
  questions: UserQuestion[],
  toolResult?: ToolResultContent,
): QuestionDraft {
  const emptyDraft = createEmptyQuestionDraft(questions.length);
  if (!toolResult || toolResult.is_error || typeof toolResult.content !== "string") {
    return emptyDraft;
  }

  const answerPairs = extractAnswerPairs(toolResult.content);
  return questions.map((question, index) => {
    const answerText = answerPairs.get(question.question);
    return answerText
      ? restoreQuestionAnswer(question, answerText)
      : emptyDraft[index];
  });
}

export function hasQuestionDraftContent(draft: QuestionDraft): boolean {
  return draft.some(
    (answer) => answer.selectedOptions.size > 0 || answer.customAnswer.trim(),
  );
}

export function toggleQuestionOption(
  draft: QuestionDraft,
  questionIndex: number,
  optionLabel: string,
  multiSelect: boolean,
): QuestionDraft {
  const answer = draft[questionIndex];
  if (!answer) {
    return draft;
  }

  const selectedOptions = multiSelect
    ? toggleOption(answer.selectedOptions, optionLabel)
    : new Set([optionLabel]);
  return replaceQuestionAnswer(draft, questionIndex, {
    customAnswer: multiSelect ? answer.customAnswer : "",
    selectedOptions,
  });
}

function toggleOption(
  selectedOptions: ReadonlySet<string>,
  optionLabel: string,
): Set<string> {
  const nextOptions = new Set(selectedOptions);
  if (nextOptions.has(optionLabel)) {
    nextOptions.delete(optionLabel);
  } else {
    nextOptions.add(optionLabel);
  }
  return nextOptions;
}

export function updateQuestionCustomAnswer(
  draft: QuestionDraft,
  questionIndex: number,
  customAnswer: string,
  multiSelect: boolean,
): QuestionDraft {
  const answer = draft[questionIndex];
  if (!answer) {
    return draft;
  }

  return replaceQuestionAnswer(draft, questionIndex, {
    customAnswer,
    selectedOptions: !multiSelect && customAnswer.trim()
      ? new Set()
      : answer.selectedOptions,
  });
}

function replaceQuestionAnswer(
  draft: QuestionDraft,
  questionIndex: number,
  answer: QuestionAnswerDraft,
): QuestionDraft {
  const nextDraft = [...draft];
  nextDraft[questionIndex] = answer;
  return nextDraft;
}

export function isQuestionDraftComplete(
  questions: UserQuestion[],
  draft: QuestionDraft,
): boolean {
  return questions.every((_, index) => {
    const answer = draft[index];
    return Boolean(
      answer
      && (answer.selectedOptions.size > 0 || answer.customAnswer.trim()),
    );
  });
}

export function buildQuestionAnswers(
  draft: QuestionDraft,
): UserQuestionAnswer[] {
  return draft.map((answer, questionIndex) => ({
    question_index: questionIndex,
    selected_options: [
      ...answer.selectedOptions,
      ...(answer.customAnswer.trim() ? [answer.customAnswer.trim()] : []),
    ],
  }));
}

export function countQuestionAnswers(draft: QuestionDraft): number {
  return draft.reduce(
    (count, answer) => count
      + answer.selectedOptions.size
      + (answer.customAnswer.trim() ? 1 : 0),
    0,
  );
}

export function summarizeQuestionAnswers(
  draft: QuestionDraft,
  limit = 3,
): string {
  const values = draft.flatMap((answer) => [
    ...answer.selectedOptions,
    ...(answer.customAnswer.trim() ? [answer.customAnswer.trim()] : []),
  ]);
  return values.slice(0, limit).join("、") + (values.length > limit ? "..." : "");
}

export function resolveQuestionInteractionStatus({
  failed,
  interactionDisabled,
  submitted,
  timedOut,
}: QuestionStatusInput): QuestionInteractionStatus {
  const candidates: Array<{
    active: boolean;
    status: QuestionInteractionStatus;
  }> = [
    { active: timedOut, status: "timed_out" },
    { active: failed, status: "failed" },
    { active: submitted, status: "submitted" },
    { active: interactionDisabled, status: "observer" },
  ];
  return candidates.find((candidate) => candidate.active)?.status ?? "active";
}

export function isQuestionStatusTerminal(
  status: QuestionInteractionStatus,
): boolean {
  return TERMINAL_QUESTION_STATUSES.has(status);
}

export function createQuestionScopeKey(
  toolUseId: string,
  questions: UserQuestion[],
): string {
  const questionIdentity = questions.map((question) => [
    question.question,
    question.multi_select,
    question.options.map((option) => option.label),
  ]);
  return JSON.stringify([toolUseId, questionIdentity]);
}
