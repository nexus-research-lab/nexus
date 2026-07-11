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
      onExpandedChange={controller.setIsExpanded}
      onSubmit={() => void controller.submit()}
      onToggleOption={controller.toggleOption}
      onUpdateCustomAnswer={controller.updateCustomAnswer}
      questions={questions}
      readOnly={controller.readOnly}
      status={controller.status}
      submitEnabled={controller.submitEnabled}
      totalSelected={controller.totalSelected}
    />
  );
}
