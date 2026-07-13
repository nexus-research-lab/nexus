import {
  CheckCircle,
  CheckSquare,
  Circle,
  Square,
  type LucideIcon,
} from "lucide-react";

import type { UserQuestion } from "@/types/conversation/interaction/ask-user-question";

interface QuestionCardTone {
  background: string;
  borderClassName: string;
}

export interface QuestionCardPresentation {
  customAnswerPlaceholder: string;
  hasCustomAnswer: boolean;
  hasSelection: boolean;
  isMultiSelect: boolean;
  selectedCount: number;
  selectionSummary: string;
  showCustomAnswer: boolean;
  tone: QuestionCardTone;
}

export interface QuestionOptionPresentation {
  Icon: LucideIcon;
  buttonClassName: string;
  iconClassName: string;
  labelClassName: string;
}

const CARD_TONES = {
  empty: {
    background:
      "color-mix(in srgb, var(--surface-panel-background) 84%, transparent)",
    borderClassName: "border-(--divider-subtle-color)",
  },
  selected: {
    background:
      "color-mix(in srgb, var(--surface-panel-background) 90%, var(--primary) 6%)",
    borderClassName: "border-primary/18",
  },
} as const;

const CUSTOM_ANSWER_PLACEHOLDERS = {
  multi: "可补充其他答案…",
  single: "没有合适选项时，在这里输入你的回答…",
} as const;

const OPTION_ICONS: Record<string, LucideIcon> = {
  "multi:idle": Square,
  "multi:selected": CheckSquare,
  "single:idle": Circle,
  "single:selected": CheckCircle,
};

const OPTION_STATE_STYLES = {
  idle: {
    button: "bg-transparent hover:bg-(--surface-interactive-hover-background)",
    icon: "text-muted-foreground/50",
    label: "text-foreground",
  },
  selected: {
    button: "bg-primary/4",
    icon: "text-primary",
    label: "text-primary",
  },
} as const;

const OPTION_INTERACTION_STYLES = {
  active: "",
  readOnly: "cursor-not-allowed opacity-60",
} as const;

export function projectQuestionCard(
  question: UserQuestion,
  selectedOptions: ReadonlySet<string>,
  customAnswer: string,
  readOnly: boolean,
): QuestionCardPresentation {
  const isMultiSelect = Boolean(question.multi_select);
  const trimmedCustomAnswer = customAnswer.trim();
  const answers = collectAnswerItems(selectedOptions, trimmedCustomAnswer);
  const hasSelection = answers.length > 0;
  const tone = CARD_TONES[hasSelection ? "selected" : "empty"];
  return {
    customAnswerPlaceholder:
      CUSTOM_ANSWER_PLACEHOLDERS[isMultiSelect ? "multi" : "single"],
    hasCustomAnswer: Boolean(trimmedCustomAnswer),
    hasSelection,
    isMultiSelect,
    selectedCount: answers.length,
    selectionSummary: summarizeAnswerItems(answers),
    showCustomAnswer: [!readOnly, Boolean(trimmedCustomAnswer)].some(Boolean),
    tone,
  };
}

function collectAnswerItems(
  selectedOptions: ReadonlySet<string>,
  customAnswer: string,
): string[] {
  return [
    ...selectedOptions,
    ...(customAnswer ? [customAnswer] : []),
  ];
}

function summarizeAnswerItems(items: string[]): string {
  const overflow = items.length > 2 ? "..." : "";
  return `${items.slice(0, 2).join("、")}${overflow}`;
}

export function projectQuestionOption(
  isMultiSelect: boolean,
  isSelected: boolean,
  readOnly: boolean,
): QuestionOptionPresentation {
  const mode = resolveOptionMode(isMultiSelect);
  const state = resolveOptionState(isSelected);
  const interaction = resolveOptionInteraction(readOnly);
  const styles = OPTION_STATE_STYLES[state];
  return {
    Icon: OPTION_ICONS[`${mode}:${state}`],
    buttonClassName: `${styles.button} ${OPTION_INTERACTION_STYLES[interaction]}`,
    iconClassName: styles.icon,
    labelClassName: styles.label,
  };
}

function resolveOptionMode(isMultiSelect: boolean): "multi" | "single" {
  return isMultiSelect ? "multi" : "single";
}

function resolveOptionState(isSelected: boolean): "idle" | "selected" {
  return isSelected ? "selected" : "idle";
}

function resolveOptionInteraction(readOnly: boolean): "active" | "readOnly" {
  return readOnly ? "readOnly" : "active";
}
