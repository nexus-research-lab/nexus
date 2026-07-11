import { useMemo } from "react";

import type {
  AskUserQuestionInput,
  UserQuestionAnswer,
} from "@/types/conversation/ask-user-question";
import type {
  ToolResultContent,
  ToolUseContent,
} from "@/types/conversation/message";

import { normalizeQuestion } from "./ask-user-question-model";
import { AskUserQuestionView } from "./ask-user-question-view";
import { useAskUserQuestionController } from "./use-ask-user-question-controller";

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
  const input = toolUse.input as AskUserQuestionInput;
  const questions = useMemo(
    () => (input?.questions ?? []).map(normalizeQuestion),
    [input?.questions],
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
