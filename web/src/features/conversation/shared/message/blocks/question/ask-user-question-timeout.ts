import type { ToolResultContent } from "@/types/conversation/message/content";

const ASK_USER_QUESTION_TIMEOUT_ERROR_CODE = "permission_request_timeout";

export function isAskUserQuestionTimedOutResult(
  toolResult?: Pick<ToolResultContent, "is_error" | "error_code"> | null,
): boolean {
  return Boolean(
    toolResult?.is_error
    && toolResult.error_code === ASK_USER_QUESTION_TIMEOUT_ERROR_CODE,
  );
}
