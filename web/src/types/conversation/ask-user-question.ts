import type { ToolResultContent } from "./message";

export interface QuestionOption {
  label: string;
  description?: string;
}

export interface UserQuestion {
  question: string;
  header?: string;
  multi_select?: boolean;
  // SDK 工具输入可能未经后端字段归一化直接到达前端。
  multiSelect?: boolean;
  options: QuestionOption[];
}

export interface AskUserQuestionInput {
  questions: UserQuestion[];
}

export interface UserQuestionAnswer {
  question_index: number;
  selected_options: string[];
}

const ASK_USER_QUESTION_TIMEOUT_ERROR_CODE = "permission_request_timeout";

export function isAskUserQuestionTimedOutResult(
  toolResult?: Pick<ToolResultContent, "is_error" | "error_code"> | null,
): boolean {
  return Boolean(
    toolResult?.is_error
    && toolResult.error_code === ASK_USER_QUESTION_TIMEOUT_ERROR_CODE,
  );
}
