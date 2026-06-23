import type {
  UserQuestion,
} from "@/types/conversation/ask-user-question";
import type { ToolResultContent } from "@/types/conversation/message";

export type QuestionSelectionState = {
  selections: Map<number, Set<string>>;
  custom_answers: Map<number, string>;
};

export function normalize_question(question: UserQuestion): UserQuestion {
  return {
    ...question,
    // 兼容 SDK 直接透传的 camelCase 字段，组件内部统一使用 snake_case。
    multi_select: question.multi_select ?? question.multiSelect ?? false,
  };
}

export function create_empty_question_selection_state(
  questions: UserQuestion[],
): QuestionSelectionState {
  const selections = new Map<number, Set<string>>();
  const custom_answers = new Map<number, string>();
  questions.forEach((_, index) => {
    selections.set(index, new Set());
    custom_answers.set(index, "");
  });
  return { selections, custom_answers };
}

function extract_answer_pairs_from_tool_result_content(
  content: string,
): Map<string, string> {
  const pairs = new Map<string, string>();
  const matcher = /"([^"]+)"="([^"]*)"/g;
  let match: RegExpExecArray | null = matcher.exec(content);
  while (match) {
    const [, question_text, answer_text] = match;
    pairs.set(question_text, answer_text);
    match = matcher.exec(content);
  }
  return pairs;
}

export function build_submitted_selection_state(
  questions: UserQuestion[],
  tool_result?: ToolResultContent,
): QuestionSelectionState {
  const empty_state = create_empty_question_selection_state(questions);
  if (!tool_result || tool_result.is_error || typeof tool_result.content !== "string") {
    return empty_state;
  }

  const answer_pairs = extract_answer_pairs_from_tool_result_content(tool_result.content);
  if (answer_pairs.size === 0) {
    return empty_state;
  }

  questions.forEach((question, index) => {
    const answer_text = answer_pairs.get(question.question);
    if (!answer_text) {
      return;
    }

    const normalized_question = normalize_question(question);
    const option_labels = new Set(normalized_question.options.map((option) => option.label));

    if (normalized_question.multi_select) {
      const answer_items = answer_text
        .split(", ")
        .map((item) => item.trim())
        .filter(Boolean);
      const selected_options = answer_items.filter((item) => option_labels.has(item));
      const custom_items = answer_items.filter((item) => !option_labels.has(item));
      empty_state.selections.set(index, new Set(selected_options));
      empty_state.custom_answers.set(index, custom_items.join(", "));
      return;
    }

    if (option_labels.has(answer_text)) {
      empty_state.selections.set(index, new Set([answer_text]));
      empty_state.custom_answers.set(index, "");
      return;
    }

    empty_state.selections.set(index, new Set());
    empty_state.custom_answers.set(index, answer_text);
  });

  return empty_state;
}

export function has_selection_state_content(state: QuestionSelectionState): boolean {
  return Array.from(state.selections.values()).some((values) => values.size > 0)
    || Array.from(state.custom_answers.values()).some((value) => value.trim().length > 0);
}
