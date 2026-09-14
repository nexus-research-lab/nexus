/**
 * INPUT: 单个结构化问题、当前回答草稿与编辑动作。
 * OUTPUT: 独立原生选项组与有界自定义回答；主选项/说明使用共享层级，草稿规则归模型。
 * POS: AskUserQuestion 交互面的单问题视图。
 */
import { useId, useRef } from "react";
import { useTextareaHeight } from "@/shared/lib/react/use-textarea-height";
import { Check, PencilLine } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  const radioGroup = useId();
  const isMultiSelect = Boolean(question.multi_select);
  const hasCustomAnswer = Boolean(customAnswer.trim());
  const showCustomAnswer = !readOnly || hasCustomAnswer;

  return (
    <fieldset
      className="ask-user-question-item min-w-0 [overflow-wrap:anywhere]"
      data-question-index={questionIndex}
      disabled={readOnly}
    >
      <legend className="mb-1.5 w-full px-0.5">
        <span className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
          {questionCount > 1 ? (
            <span
              className={cn(
                "shrink-0 tabular-nums",
                getUiTypographyClassName({ role: "metadata", tone: "soft", weight: "medium" }),
              )}
            >
              {String(questionIndex + 1).padStart(2, "0")}
            </span>
          ) : null}
          {question.header ? (
            <span
              className={cn(
                "min-w-0",
                getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }),
              )}
            >
              {question.header}
            </span>
          ) : null}
          <span
            className={cn(
              "min-w-0",
              getUiTypographyClassName({ role: "body", tone: "strong", weight: "medium" }),
            )}
          >
            {question.question}
          </span>
          {isMultiSelect ? (
            <span
              className={cn(
                "shrink-0",
                getUiTypographyClassName({ role: "metadata", tone: "soft" }),
              )}
            >
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
                name={radioGroup}
                onChange={() => onToggleOption(option.label)}
                type={isMultiSelect ? "checkbox" : "radio"}
                value={option.label}
              />
              <span
                aria-hidden
                className={cn(
                  "ask-user-question-option-indicator flex h-7 w-7 shrink-0 items-center justify-center tabular-nums sm:h-6 sm:w-6",
                  getUiTypographyClassName({ role: "metadata", weight: "medium" }),
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
                <span
                  className={getUiTypographyClassName({
                    role: "control",
                    tone: "strong",
                    weight: "medium",
                  })}
                >
                  {option.label}
                </span>
                {option.description ? (
                  <span
                    className={cn(
                      "mt-0.5 block sm:mt-0",
                      getUiTypographyClassName({ role: "supporting", tone: "muted" }),
                    )}
                  >
                    {option.description}
                  </span>
                ) : null}
              </span>
              {isSelected && !isMultiSelect ? (
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
            <QuestionCustomAnswerInput
              customAnswer={customAnswer}
              isMultiSelect={isMultiSelect}
              onCustomAnswerChange={onCustomAnswerChange}
              readOnly={readOnly}
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

function QuestionCustomAnswerInput({
  customAnswer,
  isMultiSelect,
  onCustomAnswerChange,
  readOnly,
}: {
  customAnswer: string;
  isMultiSelect: boolean;
  onCustomAnswerChange: (answer: string) => void;
  readOnly: boolean;
}) {
  const { t } = useI18n();
  const customAnswerRef = useRef<HTMLTextAreaElement>(null);
  useTextareaHeight(customAnswerRef, customAnswer, { minHeight: 24, maxHeight: 96 });
  return (
    <textarea
      ref={customAnswerRef}
      aria-label={t("composer.question_custom_answer_label")}
      className={cn(
        "min-h-6 max-h-24 min-w-0 flex-1 resize-none border-0 bg-transparent p-0 outline-none shadow-none ring-0 placeholder:text-(--text-soft) focus:border-0 focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0",
        getUiTypographyClassName({ role: "body", tone: "strong" }),
      )}
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
  );
}
