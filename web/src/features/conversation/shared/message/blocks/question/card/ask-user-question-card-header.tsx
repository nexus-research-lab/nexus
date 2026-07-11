import { ChevronDown, ChevronRight } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

import type { QuestionCardPresentation } from "./ask-user-question-card-model";

interface AskUserQuestionCardHeaderProps {
  expanded: boolean;
  onToggle: () => void;
  presentation: QuestionCardPresentation;
  question: UserQuestion;
  questionIndex: number;
}

export function AskUserQuestionCardHeader({
  expanded,
  onToggle,
  presentation,
  question,
  questionIndex,
}: AskUserQuestionCardHeaderProps) {
  const ExpandIcon = expanded ? ChevronDown : ChevronRight;
  return (
    <button
      className={cn(
        "message-cjk-font flex w-full cursor-pointer select-none items-center gap-2 px-3 py-2 text-left transition duration-(--motion-duration-fast) ease-out",
        expanded
          ? "border-b border-(--divider-subtle-color)"
          : "hover:bg-(--surface-interactive-hover-background)",
      )}
      onClick={onToggle}
      type="button"
    >
      <QuestionIndex
        hasSelection={presentation.hasSelection}
        questionIndex={questionIndex}
      />
      <QuestionHeaderLabel header={question.header} />
      <span className="flex-1 truncate text-[13px] font-medium leading-tight text-foreground">
        {question.question}
      </span>
      <MultiSelectLabel visible={presentation.isMultiSelect} />
      <CollapsedSelectionSummary
        expanded={expanded}
        hasSelection={presentation.hasSelection}
        summary={presentation.selectionSummary}
      />
      <SelectedCount
        count={presentation.selectedCount}
        visible={presentation.hasSelection}
      />
      <ExpandIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/40" />
    </button>
  );
}

function QuestionIndex({
  hasSelection,
  questionIndex,
}: {
  hasSelection: boolean;
  questionIndex: number;
}) {
  return (
    <span
      className={cn(
        "shrink-0 text-[10px] font-semibold tabular-nums tracking-[0.12em] text-(--text-soft)",
        hasSelection && "text-primary",
      )}
    >
      {String(questionIndex + 1).padStart(2, "0")}
    </span>
  );
}

function QuestionHeaderLabel({ header }: { header?: string }) {
  if (!header) {
    return null;
  }
  return (
    <span className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.14em] text-primary/80">
      {header}
    </span>
  );
}

function MultiSelectLabel({ visible }: { visible: boolean }) {
  if (!visible) {
    return null;
  }
  return <span className="text-[10px] text-muted-foreground">(多选)</span>;
}

function CollapsedSelectionSummary({
  expanded,
  hasSelection,
  summary,
}: {
  expanded: boolean;
  hasSelection: boolean;
  summary: string;
}) {
  if (expanded || !hasSelection) {
    return null;
  }
  return (
    <span className="max-w-[120px] truncate text-xs text-primary/70">
      {summary}
    </span>
  );
}

function SelectedCount({ count, visible }: { count: number; visible: boolean }) {
  if (!visible) {
    return null;
  }
  return (
    <span className="shrink-0 text-[10px] font-semibold text-primary/80">
      {count} 项
    </span>
  );
}
