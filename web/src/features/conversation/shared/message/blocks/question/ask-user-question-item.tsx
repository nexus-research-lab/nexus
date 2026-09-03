/**
 * INPUT: 单个结构化问题、当前回答草稿与编辑动作。
 * OUTPUT: 无嵌套卡片的行式选项和内联自定义回答。
 * POS: AskUserQuestion 交互面的单问题视图。
 */
import { Check, PencilLine } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

interface AskUserQuestionItemProps {
  customAnswer: string;
  onCustomAnswerChange: (customAnswer: string) => void;
  onToggleOption: (optionLabel: string) => void;
  question: UserQuestion;
  questionCount: number;
  questionIndex: number;
  readOnly: boolean;
  selectedOptions: ReadonlySet<string>;
}

export function AskUserQuestionItem({
  customAnswer,
  onCustomAnswerChange,
  onToggleOption,
  question,
  questionCount,
  questionIndex,
  readOnly,
  selectedOptions,
}: AskUserQuestionItemProps) {
  const { t } = useI18n();
  const isMultiSelect = Boolean(question.multi_select);
  const hasCustomAnswer = Boolean(customAnswer.trim());
  const showCustomAnswer = !readOnly || hasCustomAnswer;

  return (
    <fieldset
      className="ask-user-question-item min-w-0"
      data-question-index={questionIndex}
      disabled={readOnly}
    >
      <legend className="mb-1.5 w-full px-0.5">
        <span className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
          {questionCount > 1 ? (
            <span className="shrink-0 text-2xs font-medium tabular-nums text-(--text-soft)">
              {String(questionIndex + 1).padStart(2, "0")}
            </span>
          ) : null}
          {question.header ? (
            <span className="shrink-0 text-xs font-medium text-(--text-muted)">
              {question.header}
            </span>
          ) : null}
          <span className="min-w-0 text-md font-medium leading-6 text-(--text-strong)">
            {question.question}
          </span>
          {isMultiSelect ? (
            <span className="shrink-0 text-xs text-(--text-soft)">
              {t("composer.question_multi_select")}
            </span>
          ) : null}
        </span>
      </legend>

      <div className="ask-user-question-options">
        {question.options.map((option, optionIndex) => {
          const isSelected = selectedOptions.has(option.label);
          return (
            <label
              className="ask-user-question-option flex min-h-11 cursor-pointer items-center gap-2.5 px-2.5 py-2 sm:min-h-10 sm:py-1.5"
              data-read-only={readOnly}
              data-selected={isSelected}
              key={option.label}
            >
              <input
                checked={isSelected}
                className="sr-only"
                name={`ask-user-question-${questionIndex}`}
                onChange={() => onToggleOption(option.label)}
                type={isMultiSelect ? "checkbox" : "radio"}
                value={option.label}
              />
              <span
                aria-hidden
                className={cn(
                  "ask-user-question-option-indicator flex h-7 w-7 shrink-0 items-center justify-center text-xs font-medium tabular-nums sm:h-6 sm:w-6",
                  isMultiSelect ? "radius-control-xs" : "rounded-full",
                )}
              >
                {isMultiSelect
                  ? isSelected
                    ? <Check className="h-3.5 w-3.5" strokeWidth={2.4} />
                    : null
                  : optionIndex + 1}
              </span>
              <span className="min-w-0 flex-1 sm:flex sm:flex-wrap sm:items-baseline sm:gap-x-2">
                <span className="block text-sm font-medium leading-5 text-(--text-strong)">
                  {option.label}
                </span>
                {option.description ? (
                  <span className="mt-0.5 block text-xs leading-5 text-(--text-muted) sm:mt-0">
                    {option.description}
                  </span>
                ) : null}
              </span>
              {isSelected ? (
                <Check
                  aria-hidden
                  className="ask-user-question-option-check h-4 w-4 shrink-0 text-(--text-muted)"
                  strokeWidth={2.2}
                />
              ) : null}
            </label>
          );
        })}

        {showCustomAnswer ? (
          <label
            className="ask-user-question-custom-answer flex min-h-11 cursor-text items-center gap-2.5 px-2.5 py-2 sm:min-h-10 sm:py-1.5"
            data-read-only={readOnly}
            data-selected={hasCustomAnswer}
          >
            <span
              aria-hidden
              className="ask-user-question-custom-answer-icon radius-control-xs flex h-7 w-7 shrink-0 items-center justify-center sm:h-6 sm:w-6"
            >
              <PencilLine className="h-3.5 w-3.5" />
            </span>
            <textarea
              aria-label={t("composer.question_custom_answer_label")}
              className="min-h-6 max-h-24 min-w-0 flex-1 resize-none border-0 bg-transparent p-0 text-sm leading-6 text-(--text-strong) outline-none shadow-none ring-0 placeholder:text-(--text-soft) focus:border-0 focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0"
              disabled={readOnly}
              onChange={(event) => onCustomAnswerChange(event.target.value)}
              placeholder={t(
                isMultiSelect
                  ? "composer.question_custom_answer_multi_placeholder"
                  : "composer.question_custom_answer_single_placeholder",
              )}
              rows={1}
              value={customAnswer}
            />
            {hasCustomAnswer ? (
              <Check
                aria-hidden
                className="ask-user-question-custom-answer-check h-4 w-4 shrink-0 text-(--text-muted)"
                strokeWidth={2.2}
              />
            ) : null}
          </label>
        ) : null}
      </div>
    </fieldset>
  );
}
