import { Check, Loader2, Send } from "lucide-react";

import { cn } from "@/lib/utils";
import type { UserQuestion } from "@/types/conversation/ask-user-question";

import { MessageRail } from "../../ui/message-rail";
import { AskUserQuestionCard } from "./ask-user-question-card";
import { AskUserQuestionHeader } from "./ask-user-question-header";
import type {
  QuestionDraft,
  QuestionInteractionStatus,
} from "./ask-user-question-model";

interface AskUserQuestionViewProps {
  answerSummary: string;
  draft: QuestionDraft;
  draftComplete: boolean;
  expanded: boolean;
  isReady: boolean;
  isSubmitting: boolean;
  onExpandedChange: (expanded: boolean) => void;
  onSubmit: () => void;
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
  draft,
  draftComplete,
  expanded,
  isReady,
  isSubmitting,
  onExpandedChange,
  onSubmit,
  onToggleOption,
  onUpdateCustomAnswer,
  questions,
  readOnly,
  status,
  submitEnabled,
  totalSelected,
}: AskUserQuestionViewProps) {
  return (
    <MessageRail className="my-1.5">
      <AskUserQuestionHeader
        answerSummary={answerSummary}
        expanded={expanded}
        onToggle={() => onExpandedChange(!expanded)}
        questionCount={questions.length}
        readOnly={readOnly}
        status={status}
        totalSelected={totalSelected}
      />

      {expanded ? (
        <div className="mt-2 space-y-2">
          {questions.map((question, index) => {
            const answer = draft[index];
            return (
              <AskUserQuestionCard
                customAnswer={answer?.customAnswer ?? ""}
                initiallyExpanded={!readOnly}
                key={`${question.header ?? "question"}:${question.question}`}
                onCustomAnswerChange={onUpdateCustomAnswer}
                onToggleOption={onToggleOption}
                question={question}
                questionIndex={index}
                readOnly={readOnly}
                selectedOptions={answer?.selectedOptions ?? EMPTY_SELECTION}
              />
            );
          })}
        </div>
      ) : null}

      {!readOnly && expanded ? (
        <QuestionSubmitAction
          draftComplete={draftComplete}
          isReady={isReady}
          isSubmitting={isSubmitting}
          onSubmit={onSubmit}
          submitEnabled={submitEnabled}
        />
      ) : null}

      {status === "submitted" && expanded ? (
        <div className="message-cjk-font mt-2 flex items-center gap-2 border-t border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] pt-2 text-xs font-semibold text-(--success)">
          <Check className="h-3.5 w-3.5" />
          <span className="text-xs font-medium">已收到你的回应</span>
        </div>
      ) : null}
    </MessageRail>
  );
}

const EMPTY_SELECTION = new Set<string>();

interface QuestionSubmitActionProps {
  draftComplete: boolean;
  isReady: boolean;
  isSubmitting: boolean;
  onSubmit: () => void;
  submitEnabled: boolean;
}

function QuestionSubmitAction({
  draftComplete,
  isReady,
  isSubmitting,
  onSubmit,
  submitEnabled,
}: QuestionSubmitActionProps) {
  const hint = resolveSubmitHint({ draftComplete, isReady, isSubmitting });
  const SubmitIcon = isSubmitting ? Loader2 : Send;

  return (
    <div className="message-cjk-font mt-2 flex min-h-0 items-center justify-between gap-3 border-t border-(--divider-subtle-color) pt-2">
      <span className="text-[11px] leading-none text-muted-foreground">
        {hint}
      </span>
      <button
        className={cn(
          "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-[8px] border px-2.5 py-1 text-xs font-medium leading-none transition-colors",
          submitEnabled
            ? "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12"
            : "border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
        )}
        disabled={!submitEnabled}
        onClick={(event) => {
          event.stopPropagation();
          onSubmit();
        }}
        type="button"
      >
        <SubmitIcon className={cn("h-3 w-3", isSubmitting && "animate-spin")} />
        继续协作
      </button>
    </div>
  );
}

function resolveSubmitHint({
  draftComplete,
  isReady,
  isSubmitting,
}: Omit<QuestionSubmitActionProps, "onSubmit" | "submitEnabled">): string {
  const candidates = [
    { active: isSubmitting, label: "正在提交回应" },
    { active: !isReady, label: "等待提问就绪" },
    { active: draftComplete, label: "所有问题都已回应" },
  ];
  return candidates.find((candidate) => candidate.active)?.label
    ?? "每个问题至少回应一次";
}
