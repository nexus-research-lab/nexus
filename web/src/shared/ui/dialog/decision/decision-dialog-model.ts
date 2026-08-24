/**
 * INPUT: 通用确认框的语气与输入框键盘事件。
 * OUTPUT: 只决定操作按钮语气和明确的提交动作，不生成通用风险套话。
 * POS: 决策弹窗的无 UI 规则层；具体后果必须由业务调用方表达。
 */

export type ConfirmDialogVariant = "danger" | "default";

export interface ConfirmDialogPresentation {
  actionTone: "danger" | "primary";
}

const CONFIRM_PRESENTATION_BY_VARIANT: Readonly<Record<
  ConfirmDialogVariant,
  ConfirmDialogPresentation
>> = {
  danger: {
    actionTone: "danger",
  },
  default: {
    actionTone: "primary",
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
