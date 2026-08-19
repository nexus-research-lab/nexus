/**
 * INPUT: 单个 AskUserQuestion 问题及其回答草稿。
 * OUTPUT: 可展开、可选择并支持自定义回答的原生卡片。
 * POS: 问题卡片视图；新手引导模板仅改变呈现，不执行业务动作。
 */
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
  onCustomAnswerSubmit: (questionIndex: number, customAnswer: string) => void;
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
  onCustomAnswerSubmit,
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
        presentation.isOnboarding
          && "rounded-[14px] border-primary/25 shadow-[0_14px_36px_color-mix(in_srgb,var(--primary)_13%,transparent)]",
      )}
      data-onboarding-card={presentation.isOnboarding ? "true" : undefined}
      style={{
        background: presentation.isOnboarding
          ? "linear-gradient(145deg, color-mix(in srgb, var(--surface-panel-background) 88%, var(--primary) 12%), color-mix(in srgb, var(--surface-panel-background) 96%, transparent))"
          : presentation.tone.background,
      }}
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
        onCustomAnswerSubmit={(answer) =>
          onCustomAnswerSubmit(questionIndex, answer)}
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
