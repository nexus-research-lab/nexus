/**
 * INPUT: 结构化问题草稿、交互状态与提交/拒绝动作。
 * OUTPUT: 与 Composer 权限确认同层级的单面板问答视图。
 * POS: AskUserQuestion 的纯视图编排入口。
 */
import {
  AlertCircle,
  CheckCircle2,
  Loader2,
  MessageSquare,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

import { AskUserQuestionItem } from "./ask-user-question-item";
import type {
  QuestionDraft,
  QuestionInteractionStatus,
} from "./ask-user-question-model";

import "./ask-user-question.css";

interface AskUserQuestionViewProps {
  answerSummary: string;
  draft: QuestionDraft;
  draftComplete: boolean;
  expanded: boolean;
  isReady: boolean;
  isSubmitting: boolean;
  onDeny?: () => void;
  onSubmit: () => void;
  onToggleOption: (questionIndex: number, optionLabel: string) => void;
  onUpdateCustomAnswer: (questionIndex: number, customAnswer: string) => void;
  questions: UserQuestion[];
  readOnly: boolean;
  status: QuestionInteractionStatus;
  submitEnabled: boolean;
}

export function AskUserQuestionView({
  answerSummary,
  draft,
  draftComplete,
  expanded,
  isReady,
  isSubmitting,
  onDeny,
  onSubmit,
  onToggleOption,
  onUpdateCustomAnswer,
  questions,
  readOnly,
  status,
  submitEnabled,
}: AskUserQuestionViewProps) {
  return (
    <div
      aria-busy={isSubmitting}
      className="ask-user-question min-w-0"
    >
      {expanded ? (
        <QuestionList
          draft={draft}
          onToggleOption={onToggleOption}
          onUpdateCustomAnswer={onUpdateCustomAnswer}
          questions={questions}
          readOnly={readOnly}
        />
      ) : (
        <QuestionResolution
          answerSummary={answerSummary}
          status={status}
        />
      )}
      {expanded ? (
        <QuestionDecisionRow
          draftComplete={draftComplete}
          isReady={isReady}
          isSubmitting={isSubmitting}
          onDeny={onDeny}
          onSubmit={onSubmit}
          readOnly={readOnly}
          submitEnabled={submitEnabled}
        />
      ) : null}
    </div>
  );
}

const EMPTY_SELECTION = new Set<string>();
const EMPTY_ANSWER = {
  customAnswer: "",
  selectedOptions: EMPTY_SELECTION,
};

function QuestionList({
  draft,
  onToggleOption,
  onUpdateCustomAnswer,
  questions,
  readOnly,
}: {
  draft: QuestionDraft;
  onToggleOption: (questionIndex: number, optionLabel: string) => void;
  onUpdateCustomAnswer: (questionIndex: number, customAnswer: string) => void;
  questions: UserQuestion[];
  readOnly: boolean;
}) {
  return (
    <div className="ask-user-question-list">
      {questions.map((question, index) => {
        const answer = draft[index] ?? EMPTY_ANSWER;
        const keyPrefix = question.header || "question";
        return (
          <AskUserQuestionItem
            customAnswer={answer.customAnswer}
            key={`${keyPrefix}:${question.question}`}
            onCustomAnswerChange={(customAnswer) =>
              onUpdateCustomAnswer(index, customAnswer)}
            onToggleOption={(optionLabel) =>
              onToggleOption(index, optionLabel)}
            question={question}
            questionCount={questions.length}
            questionIndex={index}
            readOnly={readOnly}
            selectedOptions={answer.selectedOptions}
          />
        );
      })}
    </div>
  );
}

interface QuestionDecisionRowProps {
  draftComplete: boolean;
  isReady: boolean;
  isSubmitting: boolean;
  onDeny?: () => void;
  onSubmit: () => void;
  readOnly: boolean;
  submitEnabled: boolean;
}

function QuestionDecisionRow({
  draftComplete,
  isReady,
  isSubmitting,
  onDeny,
  onSubmit,
  readOnly,
  submitEnabled,
}: QuestionDecisionRowProps) {
  const { t } = useI18n();
  const hint = resolveSubmitHint({
    draftComplete,
    isReady,
    isSubmitting,
    readOnly,
    t,
  });

  return (
    <div className="mt-2.5 flex flex-wrap items-center justify-between gap-2.5">
      <span className="min-w-0 flex-1 text-xs leading-5 text-(--text-soft)">
        {hint}
      </span>
      <div className="flex shrink-0 items-center gap-2">
        {onDeny ? (
          <button
            className="radius-control-sm inline-flex h-9 items-center justify-center border border-(--divider-subtle-color) bg-transparent px-3.5 text-sm font-medium text-(--text-default) transition-colors hover:bg-(--interaction-hover-background) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/28 disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) sm:h-8"
            disabled={readOnly || isSubmitting}
            onClick={(event) => {
              event.stopPropagation();
              onDeny();
            }}
            type="button"
          >
            {t("composer.permission_deny")}
          </button>
        ) : null}
        <button
          className="radius-control-sm inline-flex h-9 items-center justify-center gap-2 bg-(--text-strong) px-3.5 text-sm font-medium text-(--primary-foreground) transition-opacity hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/32 disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) sm:h-8"
          disabled={!submitEnabled}
          onClick={(event) => {
            event.stopPropagation();
            onSubmit();
          }}
          type="button"
        >
          {isSubmitting ? (
            <Loader2
              aria-hidden
              className={getUiSpinnerClassName({ size: "md" })}
            />
          ) : null}
          {t("composer.question_submit")}
        </button>
      </div>
    </div>
  );
}

type Translate = ReturnType<typeof useI18n>["t"];

function resolveSubmitHint({
  draftComplete,
  isReady,
  isSubmitting,
  readOnly,
  t,
}: Omit<QuestionDecisionRowProps, "onDeny" | "onSubmit" | "submitEnabled"> & {
  t: Translate;
}): string {
  const candidates = [
    {
      active: isSubmitting,
      label: t("composer.question_hint_submitting"),
    },
    {
      active: readOnly,
      label: t("composer.question_hint_read_only"),
    },
    {
      active: !isReady,
      label: t("composer.question_hint_waiting"),
    },
    {
      active: draftComplete,
      label: t("composer.question_hint_complete"),
    },
  ];
  return candidates.find((candidate) => candidate.active)?.label
    ?? t("composer.question_hint_required");
}

interface QuestionResolutionPresentation {
  Icon: LucideIcon;
  labelKey:
    | "composer.question_status_active"
    | "composer.question_status_failed"
    | "composer.question_status_observer"
    | "composer.question_status_submitted"
    | "composer.question_status_timed_out";
  toneClassName: string;
}

const QUESTION_RESOLUTION_PRESENTATIONS: Record<
  QuestionInteractionStatus,
  QuestionResolutionPresentation
> = {
  active: {
    Icon: MessageSquare,
    labelKey: "composer.question_status_active",
    toneClassName: "text-(--text-muted)",
  },
  failed: {
    Icon: AlertCircle,
    labelKey: "composer.question_status_failed",
    toneClassName: "text-(--warning)",
  },
  observer: {
    Icon: MessageSquare,
    labelKey: "composer.question_status_observer",
    toneClassName: "text-(--text-muted)",
  },
  submitted: {
    Icon: CheckCircle2,
    labelKey: "composer.question_status_submitted",
    toneClassName: "text-(--success)",
  },
  timed_out: {
    Icon: AlertCircle,
    labelKey: "composer.question_status_timed_out",
    toneClassName: "text-(--warning)",
  },
};

function QuestionResolution({
  answerSummary,
  status,
}: {
  answerSummary: string;
  status: QuestionInteractionStatus;
}) {
  const { t } = useI18n();
  const presentation = QUESTION_RESOLUTION_PRESENTATIONS[status];
  const { Icon } = presentation;
  return (
    <div className="flex min-w-0 items-center gap-2 py-1 text-sm">
      <Icon
        aria-hidden
        className={cn("h-4 w-4 shrink-0", presentation.toneClassName)}
      />
      <span className="shrink-0 font-medium text-(--text-default)">
        {t(presentation.labelKey)}
      </span>
      {answerSummary ? (
        <span className="truncate text-(--text-muted)">
          {answerSummary}
        </span>
      ) : null}
    </div>
  );
}
