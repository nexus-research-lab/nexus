import { useState } from "react";
import {
  CheckCircle,
  CheckSquare,
  ChevronDown,
  ChevronRight,
  Circle,
  Square,
} from "lucide-react";

import { cn } from "@/lib/utils";
import type { UserQuestion } from "@/types/conversation/ask-user-question";

interface AskUserQuestionCardProps {
  question: UserQuestion;
  question_index: number;
  selected_options: Set<string>;
  custom_answer: string;
  on_toggle_option: (question_index: number, option_label: string, multi_select: boolean) => void;
  on_custom_answer_change: (question_index: number, custom_answer: string, multi_select: boolean) => void;
  is_submitted: boolean;
  default_expanded?: boolean;
}

/** 单个问题卡片（支持独立收起） */
export function AskUserQuestionCard({
  question,
  question_index,
  selected_options,
  custom_answer,
  on_toggle_option,
  on_custom_answer_change,
  is_submitted,
  default_expanded = false,
}: AskUserQuestionCardProps) {
  const [isExpanded, setIsExpanded] = useState(default_expanded);
  const isMultiSelect = question.multi_select ?? false;
  const hasCustomAnswer = custom_answer.trim().length > 0;
  const showCustomAnswer = !is_submitted || hasCustomAnswer;
  const hasSelection = selected_options.size > 0 || hasCustomAnswer;
  const selectedCount = selected_options.size + (hasCustomAnswer ? 1 : 0);
  const summaryItems = [...Array.from(selected_options), ...(hasCustomAnswer ? [custom_answer.trim()] : [])];
  const selectionSummary = summaryItems.slice(0, 2).join("、") +
    (summaryItems.length > 2 ? "..." : "");

  return (
    <div
      className={cn(
        "overflow-hidden rounded-[10px] border transition duration-(--motion-duration-fast) ease-out",
        hasSelection
          ? "border-primary/18"
          : "border-(--divider-subtle-color)",
      )}
      style={{
        background: hasSelection
          ? "color-mix(in srgb, var(--surface-panel-background) 90%, var(--primary) 6%)"
          : "color-mix(in srgb, var(--surface-panel-background) 84%, transparent)",
      }}
    >
      <button
        type="button"
        className={cn(
          "message-cjk-font flex w-full cursor-pointer select-none items-center gap-2 px-3 py-2 text-left transition duration-(--motion-duration-fast) ease-out",
          isExpanded && "border-b border-(--divider-subtle-color)",
          !isExpanded && "hover:bg-(--surface-interactive-hover-background)",
        )}
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <span className={cn(
          "shrink-0 text-[10px] font-semibold tabular-nums tracking-[0.12em] text-(--text-soft)",
          hasSelection && "text-primary",
        )}>
          {String(question_index + 1).padStart(2, "0")}
        </span>

        {question.header && (
          <span className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.14em] text-primary/80">
            {question.header}
          </span>
        )}

        <span className="flex-1 truncate text-[13px] font-medium leading-tight text-foreground">
          {question.question}
        </span>

        {isMultiSelect && (
          <span className="text-[10px] text-muted-foreground">(多选)</span>
        )}

        {!isExpanded && hasSelection && (
          <span className="max-w-[120px] truncate text-xs text-primary/70">
            {selectionSummary}
          </span>
        )}

        {hasSelection && (
          <span className="shrink-0 text-[10px] font-semibold text-primary/80">
            {selectedCount} 项
          </span>
        )}

        <div className="text-muted-foreground/40">
          {isExpanded ? (
            <ChevronDown className="h-3.5 w-3.5" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5" />
          )}
        </div>
      </button>

      {isExpanded && (
        <div className="message-cjk-font p-2.5">
          <div className="overflow-hidden rounded-[8px] border border-(--divider-subtle-color)">
            {question.options.map((option, optIndex) => {
              const isSelected = selected_options.has(option.label);
              const Icon = isMultiSelect
                ? (isSelected ? CheckSquare : Square)
                : (isSelected ? CheckCircle : Circle);

              return (
                <button
                  key={optIndex}
                  className={cn(
                    "w-full border-b border-(--divider-subtle-color) px-3 py-2 text-left transition duration-(--motion-duration-fast) ease-out last:border-b-0",
                    isSelected
                      ? "bg-primary/4"
                      : "bg-transparent hover:bg-(--surface-interactive-hover-background)",
                    is_submitted && "cursor-not-allowed opacity-60",
                  )}
                  disabled={is_submitted}
                  onClick={(e) => {
                    e.stopPropagation();
                    on_toggle_option(question_index, option.label, isMultiSelect);
                  }}
                  type="button"
                >
                  <div className="flex items-start gap-2.5">
                    <Icon className={cn(
                      "mt-0.5 h-4 w-4 flex-shrink-0 transition-colors",
                      isSelected ? "text-primary" : "text-muted-foreground/50",
                    )} />
                    <div className="min-w-0 flex-1">
                      <div className={cn(
                        "text-[13px] font-medium leading-tight",
                        isSelected ? "text-primary" : "text-foreground",
                      )}>
                        {option.label}
                      </div>
                      {option.description && (
                        <div className="mt-1 text-[11px] leading-snug text-muted-foreground">
                          {option.description}
                        </div>
                      )}
                    </div>
                    {isSelected && (
                      <span className="shrink-0 text-[10px] font-medium text-primary/80">
                        已选
                      </span>
                    )}
                  </div>
                </button>
              );
            })}

            {showCustomAnswer ? (
              <div
                className="px-3 py-2"
                onClick={(event) => event.stopPropagation()}
                onKeyDown={(event) => event.stopPropagation()}
                role="presentation"
              >
                <div className="mb-1 flex items-center justify-between gap-2">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">自定义回答</div>
                  {hasCustomAnswer && (
                    <span className="text-[10px] font-medium text-primary/80">
                      已填写
                    </span>
                  )}
                </div>
                <div className="border-b border-(--divider-subtle-color)">
                  <textarea
                    aria-label="自定义回答"
                    className={cn(
                      "h-7 min-h-7 w-full resize-none border-0 bg-transparent px-0 py-0 text-[13px] leading-7 text-(--text-strong) outline-none shadow-none ring-0 transition duration-(--motion-duration-fast) ease-out focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                      "placeholder:text-muted-foreground/70",
                      is_submitted && "cursor-not-allowed opacity-60",
                    )}
                    disabled={is_submitted}
                    value={custom_answer}
                    onChange={(event) => {
                      on_custom_answer_change(
                        question_index,
                        event.target.value,
                        isMultiSelect,
                      );
                    }}
                    onClick={(event) => event.stopPropagation()}
                    placeholder={isMultiSelect ? "可补充其他答案…" : "没有合适选项时，在这里输入你的回答…"}
                    rows={1}
                  />
                </div>
              </div>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
