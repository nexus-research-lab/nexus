import {
  AlertCircle,
  CheckCircle,
  ChevronDown,
  ChevronRight,
  MessageSquare,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";

import type { QuestionInteractionStatus } from "./ask-user-question-model";

interface QuestionHeaderPresentation {
  Icon: LucideIcon;
  label: string;
  toneClassName: string;
}

const QUESTION_HEADER_PRESENTATIONS: Record<
  QuestionInteractionStatus,
  QuestionHeaderPresentation
> = {
  active: {
    Icon: MessageSquare,
    label: "需要你的回应",
    toneClassName: "text-primary",
  },
  failed: {
    Icon: AlertCircle,
    label: "提问未完成",
    toneClassName: "text-(--warning)",
  },
  observer: {
    Icon: MessageSquare,
    label: "等待回应",
    toneClassName: "text-primary",
  },
  submitted: {
    Icon: CheckCircle,
    label: "已收到你的回应",
    toneClassName: "text-(--success)",
  },
  timed_out: {
    Icon: AlertCircle,
    label: "提问已超时",
    toneClassName: "text-(--warning)",
  },
};

interface AskUserQuestionHeaderProps {
  answerSummary: string;
  expanded: boolean;
  onToggle: () => void;
  questionCount: number;
  readOnly: boolean;
  status: QuestionInteractionStatus;
  totalSelected: number;
}

export function AskUserQuestionHeader({
  answerSummary,
  expanded,
  onToggle,
  questionCount,
  readOnly,
  status,
  totalSelected,
}: AskUserQuestionHeaderProps) {
  const presentation = QUESTION_HEADER_PRESENTATIONS[status];
  const { Icon } = presentation;
  const ExpandIcon = expanded ? ChevronDown : ChevronRight;

  return (
    <button
      className="flex min-h-8 w-full cursor-pointer select-none items-center gap-2 py-0.5 text-left text-xs transition duration-(--motion-duration-fast) ease-out"
      onClick={onToggle}
      type="button"
    >
      <span
        className={cn(
          "flex h-5 w-5 items-center justify-center rounded-full",
          presentation.toneClassName,
        )}
        data-timeline-anchor
        data-timeline-anchor-mode="box"
      >
        <Icon className="h-3.5 w-3.5" />
      </span>

      <span className={cn(
        "font-medium uppercase tracking-[0.12em]",
        presentation.toneClassName,
      )}>
        {presentation.label}
      </span>
      <span className="text-muted-foreground/30">│</span>
      <span className="text-muted-foreground">{questionCount} 个问题</span>

      {!expanded && answerSummary ? (
        <>
          <span className="text-muted-foreground/30">│</span>
          <span className="max-w-[200px] truncate text-(--text-muted)">
            {answerSummary}
          </span>
        </>
      ) : null}

      <span className="flex-1" />
      {!readOnly && totalSelected > 0 ? (
        <span className="text-[10px] font-semibold text-primary/80">
          已选 {totalSelected} 项
        </span>
      ) : null}
      <ExpandIcon className="h-3.5 w-3.5 text-muted-foreground/40" />
    </button>
  );
}
