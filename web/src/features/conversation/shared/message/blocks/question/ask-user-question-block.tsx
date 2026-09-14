// INPUT: 已解析为工具形状的问题输入、当前响应能力和可选终态证据。
// OUTPUT: 组合问题解析、控制器与唯一问答视图；失败提交保留草稿。
// POS: Pending 适配器下的问答编排层；消息/历史不得直接挂载可回答选项。

import { useMemo } from "react";

import type { UserQuestionAnswer } from "@/types/conversation/interaction/ask-user-question";
import type {
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message/content";

import { parseAskUserQuestions } from "./ask-user-question-model";
import { AskUserQuestionView } from "./ask-user-question-view";
import { useAskUserQuestionController } from "./controller/use-ask-user-question-controller";

interface AskUserQuestionBlockProps {
  initialSubmitted?: boolean;
  interactionDisabled?: boolean;
  isReady?: boolean;
  onDeny?: () => void;
  onSubmit: (
    toolUseId: string,
    answers: UserQuestionAnswer[],
  ) => boolean | Promise<boolean>;
  toolResult?: ToolResultContent;
  toolUse: ToolUseContent;
}

export function AskUserQuestionBlock({
  initialSubmitted = false,
  interactionDisabled = false,
  isReady = true,
  onDeny,
  onSubmit,
  toolResult,
  toolUse,
}: AskUserQuestionBlockProps) {
  const questions = useMemo(
    () => parseAskUserQuestions(toolUse.input),
    [toolUse.input],
  );
  const controller = useAskUserQuestionController({
    initialSubmitted,
    interactionDisabled,
    isReady,
    onSubmit,
    questions,
    toolResult,
    toolUseId: toolUse.id,
  });

  if (questions.length === 0) {
    return null;
  }

  return (
    <AskUserQuestionView
      answerSummary={controller.answerSummary}
      draft={controller.draft}
      draftComplete={controller.draftComplete}
      expanded={controller.isExpanded}
      isReady={controller.isReady}
      isSubmitting={controller.isSubmitting}
      onDeny={onDeny}
      onSubmit={() => void controller.submit().catch((error: unknown) => {
        // 调用方拥有传输失败反馈；事件入口只处理 Promise，避免未处理拒绝。
        console.error("Question submission failed:", error);
      })}
      onToggleOption={controller.toggleOption}
      onUpdateCustomAnswer={controller.updateCustomAnswer}
      questions={questions}
      readOnly={controller.readOnly}
      status={controller.status}
      submitEnabled={controller.submitEnabled}
    />
  );
}
