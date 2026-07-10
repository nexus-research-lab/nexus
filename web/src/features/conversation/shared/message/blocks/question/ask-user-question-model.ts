import type {
  UserQuestion,
} from "@/types/conversation/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message";

export type QuestionSelectionState = {
  selections: Map<number, Set<string>>;
  custom_answers: Map<number, string>;
};

export function normalizeQuestion(question: UserQuestion): UserQuestion {
  return {
    ...question,
    // 兼容 SDK 直接透传的 camelCase 字段，组件内部统一使用 snakeCase。
    multi_select: question.multi_select ?? question.multiSelect ?? false,
  };
}

export function createEmptyQuestionSelectionState(
  questions: UserQuestion[],
): QuestionSelectionState {
  const selections = new Map<number, Set<string>>();
  const customAnswers = new Map<number, string>();
  questions.forEach((_, index) => {
    selections.set(index, new Set());
    customAnswers.set(index, "");
  });
  return { selections, custom_answers: customAnswers };
}

function extractAnswerPairsFromToolResultContent(
  content: string,
): Map<string, string> {
  const pairs = new Map<string, string>();
  const matcher = /"([^"]+)"="([^"]*)"/g;
  let match: RegExpExecArray | null = matcher.exec(content);
  while (match) {
    const [, questionText, answerText] = match;
    pairs.set(questionText, answerText);
    match = matcher.exec(content);
  }
  return pairs;
}

export function buildSubmittedSelectionState(
  questions: UserQuestion[],
  toolResult?: ToolResultContent,
): QuestionSelectionState {
  const emptyState = createEmptyQuestionSelectionState(questions);
  if (!toolResult || toolResult.is_error || typeof toolResult.content !== "string") {
    return emptyState;
  }

  const answerPairs = extractAnswerPairsFromToolResultContent(toolResult.content);
  if (answerPairs.size === 0) {
    return emptyState;
  }

  questions.forEach((question, index) => {
    const answerText = answerPairs.get(question.question);
    if (!answerText) {
      return;
    }

    const normalizedQuestion = normalizeQuestion(question);
    const optionLabels = new Set(normalizedQuestion.options.map((option) => option.label));

    if (normalizedQuestion.multi_select) {
      const answerItems = answerText
        .split(", ")
        .map((item) => item.trim())
        .filter(Boolean);
      const selectedOptions = answerItems.filter((item) => optionLabels.has(item));
      const customItems = answerItems.filter((item) => !optionLabels.has(item));
      emptyState.selections.set(index, new Set(selectedOptions));
      emptyState.custom_answers.set(index, customItems.join(", "));
      return;
    }

    if (optionLabels.has(answerText)) {
      emptyState.selections.set(index, new Set([answerText]));
      emptyState.custom_answers.set(index, "");
      return;
    }

    emptyState.selections.set(index, new Set());
    emptyState.custom_answers.set(index, answerText);
  });

  return emptyState;
}

export function hasSelectionStateContent(state: QuestionSelectionState): boolean {
  return Array.from(state.selections.values()).some((values) => values.size > 0)
    || Array.from(state.custom_answers.values()).some((value) => value.trim().length > 0);
}
