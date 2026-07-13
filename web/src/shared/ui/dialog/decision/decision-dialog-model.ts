import type { CSSProperties } from "react";

export type ConfirmDialogVariant = "danger" | "default";

export interface ConfirmDialogPresentation {
  actionTone: "danger" | "primary";
  iconStyle?: CSSProperties;
  noteTone: "danger" | "default";
  subtitle: string;
}

const CONFIRM_PRESENTATION_BY_VARIANT: Readonly<Record<
  ConfirmDialogVariant,
  ConfirmDialogPresentation
>> = {
  danger: {
    actionTone: "danger",
    iconStyle: {
      background:
        "color-mix(in srgb, var(--destructive) 12%, var(--modal-dialog-body-background))",
      border:
        "1px solid color-mix(in srgb, var(--destructive) 22%, var(--modal-card-border))",
      color: "var(--destructive)",
    },
    noteTone: "danger",
    subtitle: "此操作会立即生效，且不可恢复。",
  },
  default: {
    actionTone: "primary",
    noteTone: "default",
    subtitle: "请确认是否继续执行该操作。",
  },
};

export function getConfirmDialogPresentation(
  variant: ConfirmDialogVariant,
): ConfirmDialogPresentation {
  return CONFIRM_PRESENTATION_BY_VARIANT[variant];
}

export type PromptInputMode = "multiline" | "single";
export type PromptKeyboardAction = "ignore" | "submit";

interface PromptKeyboardContext {
  ctrlKey: boolean;
  key: string;
  metaKey: boolean;
  mode: PromptInputMode;
}

interface PromptKeyboardRule {
  action: PromptKeyboardAction;
  matches: (context: PromptKeyboardContext) => boolean;
}

const PROMPT_KEYBOARD_RULES: readonly PromptKeyboardRule[] = [
  {
    action: "submit",
    matches: ({ key, mode }) => mode === "single" && key === "Enter",
  },
  {
    action: "submit",
    matches: ({ ctrlKey, key, metaKey, mode }) => (
      mode === "multiline" && key === "Enter" && (metaKey || ctrlKey)
    ),
  },
];

export function resolvePromptKeyboardAction(
  context: PromptKeyboardContext,
): PromptKeyboardAction {
  return PROMPT_KEYBOARD_RULES.find((rule) => rule.matches(context))?.action ?? "ignore";
}
