import { isAskUserQuestionTimedOutResult } from "@/features/conversation/shared/message/blocks/question/ask-user-question-timeout";
import type { ContentBlock } from "@/types/conversation/message/content";

import { ASK_USER_QUESTION_TOOL_NAME } from "../activity/message-activity-state";

export function hasTimedOutAskUserQuestion(
  content: readonly ContentBlock[],
): boolean {
  const questionToolUseIds = new Set<string>();
  for (const block of content) {
    if (block.type === "tool_use" && block.name === ASK_USER_QUESTION_TOOL_NAME) {
      questionToolUseIds.add(block.id);
    }
  }
  return content.some(
    (block) => block.type === "tool_result"
      && questionToolUseIds.has(block.tool_use_id)
      && isAskUserQuestionTimedOutResult(block),
  );
}
