import { useState } from "react";

import { cn } from "@/shared/ui/class-name";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

import { AskUserQuestionCardBody } from "./ask-user-question-card-body";
import { AskUserQuestionCardHeader } from "./ask-user-question-card-header";
import { projectQuestionCard } from "./ask-user-question-card-model";

interface AskUserQuestionCardProps {
  customAnswer: string;
  initiallyExpanded?: boolean;
  onCustomAnswerChange: (questionIndex: number, customAnswer: string) => void;
  onToggleOption: (questionIndex: number, optionLabel: string) => void;
  question: UserQuestion;
  questionIndex: number;
  readOnly: boolean;
  selectedOptions: ReadonlySet<string>;
}

export function AskUserQuestionCard({
  customAnswer,
  initiallyExpanded = false,
  onCustomAnswerChange,
  onToggleOption,
  question,
  questionIndex,
  readOnly,
  selectedOptions,
}: AskUserQuestionCardProps) {
  const [expanded, setExpanded] = useState(initiallyExpanded);
  const presentation = projectQuestionCard(
    question,
    selectedOptions,
    customAnswer,
    readOnly,
  );
  return (
    <div
      className={cn(
        "overflow-hidden rounded-[10px] border transition duration-(--motion-duration-fast) ease-out",
        presentation.tone.borderClassName,
      )}
      style={{ background: presentation.tone.background }}
    >
      <AskUserQuestionCardHeader
        expanded={expanded}
        onToggle={() => setExpanded((current) => !current)}
        presentation={presentation}
        question={question}
        questionIndex={questionIndex}
      />
      <AskUserQuestionCardBody
        customAnswer={customAnswer}
        expanded={expanded}
        onCustomAnswerChange={(answer) =>
          onCustomAnswerChange(questionIndex, answer)}
        onToggleOption={(optionLabel) =>
          onToggleOption(questionIndex, optionLabel)}
        presentation={presentation}
        question={question}
        readOnly={readOnly}
        selectedOptions={selectedOptions}
      />
    </div>
  );
}
