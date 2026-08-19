/**
 * INPUT: AskUserQuestion 问题集合、草稿、交互状态与提交动作。
 * OUTPUT: 原生问题交互区及新手引导卡片模板外观。
 * POS: 问答内容块视图；回答状态和提交互斥由上层控制器持有。
 */
import { Check, Loader2, Send } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

import { MessageRail } from "../../ui/message-rail";
import { AskUserQuestionCard } from "./card/ask-user-question-card";
import { isOnboardingQuestion } from "./ask-user-question-model";
import { AskUserQuestionHeader } from "./ask-user-question-header";
import type {
  QuestionDraft,
  QuestionInteractionStatus,
} from "./ask-user-question-model";

interface AskUserQuestionViewProps {
  answerSummary: string;
  autoSubmit: boolean;
  draft: QuestionDraft;
  draftComplete: boolean;
  expanded: boolean;
  isReady: boolean;
  isSubmitting: boolean;
  onExpandedChange: (expanded: boolean) => void;
  onSubmit: () => void;
  onSubmitCustomAnswer: (
    questionIndex: number,
    customAnswer: string,
  ) => void;
  onToggleOption: (questionIndex: number, optionLabel: string) => void;
  onUpdateCustomAnswer: (questionIndex: number, customAnswer: string) => void;
  questions: UserQuestion[];
  readOnly: boolean;
  status: QuestionInteractionStatus;
  submitEnabled: boolean;
  totalSelected: number;
}

export function AskUserQuestionView({
  answerSummary,
  autoSubmit,
  draft,
  draftComplete,
  expanded,
  isReady,
  isSubmitting,
  onExpandedChange,
  onSubmit,
  onSubmitCustomAnswer,
  onToggleOption,
  onUpdateCustomAnswer,
  questions,
  readOnly,
  status,
  submitEnabled,
  totalSelected,
}: AskUserQuestionViewProps) {
  const isOnboarding = questions.some(isOnboardingQuestion);
  return (
    <MessageRail
      className={cn(
        "my-1.5",
        isOnboarding
          && "rounded-[18px] border border-primary/15 bg-primary/[0.025] px-3 py-3 shadow-[0_18px_48px_color-mix(in_srgb,var(--primary)_8%,transparent)]",
      )}
    >
      <AskUserQuestionHeader
        answerSummary={answerSummary}
        expanded={expanded}
        onToggle={() => onExpandedChange(!expanded)}
        questionCount={questions.length}
        readOnly={readOnly}
        status={status}
        totalSelected={totalSelected}
      />
      <QuestionCardList
        draft={draft}
        expanded={expanded}
        onSubmitCustomAnswer={onSubmitCustomAnswer}
        onToggleOption={onToggleOption}
        onUpdateCustomAnswer={onUpdateCustomAnswer}
        questions={questions}
        readOnly={readOnly}
      />
      <QuestionSubmitSection
        draftComplete={draftComplete}
        expanded={expanded}
        isReady={isReady}
        isSubmitting={isSubmitting}
        onSubmit={onSubmit}
        readOnly={readOnly}
        submitEnabled={submitEnabled}
        submitLabel={isOnboarding ? "继续引导" : "继续协作"}
        visible={!autoSubmit}
      />
      <QuestionSubmittedNotice expanded={expanded} status={status} />
    </MessageRail>
  );
}

const EMPTY_SELECTION = new Set<string>();

function QuestionCardList({
  draft,
  expanded,
  onSubmitCustomAnswer,
  onToggleOption,
  onUpdateCustomAnswer,
  questions,
  readOnly,
}: {
  draft: QuestionDraft;
  expanded: boolean;
  onSubmitCustomAnswer: (
    questionIndex: number,
    customAnswer: string,
  ) => void;
  onToggleOption: (questionIndex: number, optionLabel: string) => void;
  onUpdateCustomAnswer: (questionIndex: number, customAnswer: string) => void;
  questions: UserQuestion[];
  readOnly: boolean;
}) {
  if (!expanded) {
    return null;
  }
  return (
    <div className="mt-2 space-y-2">
      {questions.map((question, index) => {
        const answer = draft[index] ?? EMPTY_ANSWER;
        const keyPrefix = question.header || "question";
        return (
          <AskUserQuestionCard
            customAnswer={answer.customAnswer}
            initiallyExpanded={!readOnly}
            key={`${keyPrefix}:${question.question}`}
            onCustomAnswerChange={onUpdateCustomAnswer}
            onCustomAnswerSubmit={onSubmitCustomAnswer}
            onToggleOption={onToggleOption}
            question={question}
            questionIndex={index}
            readOnly={readOnly}
            selectedOptions={answer.selectedOptions}
          />
        );
      })}
    </div>
  );
}

const EMPTY_ANSWER = {
  customAnswer: "",
  selectedOptions: EMPTY_SELECTION,
};

function QuestionSubmitSection({
  draftComplete,
  expanded,
  isReady,
  isSubmitting,
  onSubmit,
  readOnly,
  submitEnabled,
  submitLabel,
  visible,
}: QuestionSubmitActionProps & {
  expanded: boolean;
  readOnly: boolean;
  visible: boolean;
}) {
  if (readOnly || !expanded || !visible) {
    return null;
  }
  return (
    <QuestionSubmitAction
      draftComplete={draftComplete}
      isReady={isReady}
      isSubmitting={isSubmitting}
      onSubmit={onSubmit}
      submitEnabled={submitEnabled}
      submitLabel={submitLabel}
    />
  );
}

function QuestionSubmittedNotice({
  expanded,
  status,
}: {
  expanded: boolean;
  status: QuestionInteractionStatus;
}) {
  if (!expanded || status !== "submitted") {
    return null;
  }
  return (
    <div className="message-cjk-font mt-2 flex items-center gap-2 border-t border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] pt-2 text-xs font-semibold text-(--success)">
      <Check className="h-3.5 w-3.5" />
      <span className="text-xs font-medium">已收到你的回应</span>
    </div>
  );
}

interface QuestionSubmitActionProps {
  draftComplete: boolean;
  isReady: boolean;
  isSubmitting: boolean;
  onSubmit: () => void;
  submitEnabled: boolean;
  submitLabel: string;
}

function QuestionSubmitAction({
  draftComplete,
  isReady,
  isSubmitting,
  onSubmit,
  submitEnabled,
  submitLabel,
}: QuestionSubmitActionProps) {
  const hint = resolveSubmitHint({ draftComplete, isReady, isSubmitting });
  const presentation = resolveSubmitPresentation(isSubmitting, submitEnabled);
  const { Icon } = presentation;

  return (
    <div className="message-cjk-font mt-2 flex min-h-0 items-center justify-between gap-3 border-t border-(--divider-subtle-color) pt-2">
      <span className="text-[11px] leading-none text-muted-foreground">
        {hint}
      </span>
      <button
        className={cn(
          "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-[8px] border px-2.5 py-1 text-xs font-medium leading-none transition-colors",
          presentation.buttonClassName,
        )}
        disabled={!submitEnabled}
        onClick={(event) => {
          event.stopPropagation();
          onSubmit();
        }}
        type="button"
      >
        <Icon className={cn("h-3 w-3", presentation.iconClassName)} />
        {submitLabel}
      </button>
    </div>
  );
}

function resolveSubmitPresentation(
  isSubmitting: boolean,
  submitEnabled: boolean,
) {
  const candidates = [
    { active: isSubmitting, state: "submitting" },
    { active: submitEnabled, state: "enabled" },
  ] as const;
  const state = candidates.find((candidate) => candidate.active)?.state
    ?? "disabled";
  return SUBMIT_PRESENTATIONS[state];
}

const SUBMIT_PRESENTATIONS = {
  disabled: {
    Icon: Send,
    buttonClassName:
      "border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
    iconClassName: "",
  },
  enabled: {
    Icon: Send,
    buttonClassName:
      "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12",
    iconClassName: "",
  },
  submitting: {
    Icon: Loader2,
    buttonClassName:
      "border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
    iconClassName: "animate-spin",
  },
} as const;

function resolveSubmitHint({
  draftComplete,
  isReady,
  isSubmitting,
}: Omit<
  QuestionSubmitActionProps,
  "onSubmit" | "submitEnabled" | "submitLabel"
>): string {
  const candidates = [
    { active: isSubmitting, label: "正在提交回应" },
    { active: !isReady, label: "等待提问就绪" },
    { active: draftComplete, label: "所有问题都已回应" },
  ];
  return candidates.find((candidate) => candidate.active)?.label
    ?? "每个问题至少回应一次";
}
