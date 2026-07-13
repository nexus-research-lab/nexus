import { cn } from "@/shared/ui/class-name";
import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

import {
  projectQuestionOption,
  type QuestionCardPresentation,
} from "./ask-user-question-card-model";

interface AskUserQuestionCardBodyProps {
  customAnswer: string;
  expanded: boolean;
  onCustomAnswerChange: (customAnswer: string) => void;
  onToggleOption: (optionLabel: string) => void;
  presentation: QuestionCardPresentation;
  question: UserQuestion;
  readOnly: boolean;
  selectedOptions: ReadonlySet<string>;
}

export function AskUserQuestionCardBody({
  customAnswer,
  expanded,
  onCustomAnswerChange,
  onToggleOption,
  presentation,
  question,
  readOnly,
  selectedOptions,
}: AskUserQuestionCardBodyProps) {
  if (!expanded) {
    return null;
  }
  return (
    <div className="message-cjk-font p-2.5">
      <div className="overflow-hidden rounded-[8px] border border-(--divider-subtle-color)">
        {question.options.map((option) => (
          <QuestionOptionButton
            description={option.description}
            isMultiSelect={presentation.isMultiSelect}
            isSelected={selectedOptions.has(option.label)}
            key={option.label}
            label={option.label}
            onToggle={onToggleOption}
            readOnly={readOnly}
          />
        ))}
        <CustomAnswerField
          customAnswer={customAnswer}
          onChange={onCustomAnswerChange}
          placeholder={presentation.customAnswerPlaceholder}
          readOnly={readOnly}
          visible={presentation.showCustomAnswer}
        />
      </div>
    </div>
  );
}

function QuestionOptionButton({
  description,
  isMultiSelect,
  isSelected,
  label,
  onToggle,
  readOnly,
}: {
  description?: string;
  isMultiSelect: boolean;
  isSelected: boolean;
  label: string;
  onToggle: (optionLabel: string) => void;
  readOnly: boolean;
}) {
  const presentation = projectQuestionOption(
    isMultiSelect,
    isSelected,
    readOnly,
  );
  const { Icon } = presentation;
  return (
    <button
      className={cn(
        "w-full border-b border-(--divider-subtle-color) px-3 py-2 text-left transition duration-(--motion-duration-fast) ease-out last:border-b-0",
        presentation.buttonClassName,
      )}
      disabled={readOnly}
      onClick={(event) => {
        event.stopPropagation();
        onToggle(label);
      }}
      type="button"
    >
      <div className="flex items-start gap-2.5">
        <Icon
          className={cn(
            "mt-0.5 h-4 w-4 flex-shrink-0 transition-colors",
            presentation.iconClassName,
          )}
        />
        <div className="min-w-0 flex-1">
          <div
            className={cn(
              "text-[13px] font-medium leading-tight",
              presentation.labelClassName,
            )}
          >
            {label}
          </div>
          <QuestionOptionDescription description={description} />
        </div>
        <SelectedOptionBadge visible={isSelected} />
      </div>
    </button>
  );
}

function QuestionOptionDescription({ description }: { description?: string }) {
  if (!description) {
    return null;
  }
  return (
    <div className="mt-1 text-[11px] leading-snug text-muted-foreground">
      {description}
    </div>
  );
}

function SelectedOptionBadge({ visible }: { visible: boolean }) {
  if (!visible) {
    return null;
  }
  return (
    <span className="shrink-0 text-[10px] font-medium text-primary/80">
      已选
    </span>
  );
}

function CustomAnswerField({
  customAnswer,
  onChange,
  placeholder,
  readOnly,
  visible,
}: {
  customAnswer: string;
  onChange: (customAnswer: string) => void;
  placeholder: string;
  readOnly: boolean;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <div
      className="px-3 py-2"
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => event.stopPropagation()}
      role="presentation"
    >
      <div className="mb-1 flex items-center justify-between gap-2">
        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--text-soft)">
          自定义回答
        </div>
        <CustomAnswerBadge visible={Boolean(customAnswer.trim())} />
      </div>
      <div className="border-b border-(--divider-subtle-color)">
        <textarea
          aria-label="自定义回答"
          className={cn(
            "h-7 min-h-7 w-full resize-none border-0 bg-transparent px-0 py-0 text-[13px] leading-7 text-(--text-strong) outline-none shadow-none ring-0 transition duration-(--motion-duration-fast) ease-out focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
            "placeholder:text-muted-foreground/70",
            readOnly && "cursor-not-allowed opacity-60",
          )}
          disabled={readOnly}
          onChange={(event) => onChange(event.target.value)}
          onClick={(event) => event.stopPropagation()}
          placeholder={placeholder}
          rows={1}
          value={customAnswer}
        />
      </div>
    </div>
  );
}

function CustomAnswerBadge({ visible }: { visible: boolean }) {
  if (!visible) {
    return null;
  }
  return (
    <span className="text-[10px] font-medium text-primary/80">已填写</span>
  );
}
