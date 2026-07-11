export type DialogKeyboardAction =
  | "close"
  | "focus-first"
  | "focus-last"
  | "focus-root"
  | "ignore";

interface DialogKeyboardContext {
  activeIndex: number;
  focusInside: boolean;
  focusableCount: number;
  hasOpenOverlay: boolean;
  key: string;
  shiftKey: boolean;
}

interface DialogKeyboardRule {
  action: DialogKeyboardAction;
  matches: (context: DialogKeyboardContext) => boolean;
}

const DIALOG_KEYBOARD_RULES: readonly DialogKeyboardRule[] = [
  {
    action: "close",
    matches: ({ hasOpenOverlay, key }) => key === "Escape" && !hasOpenOverlay,
  },
  {
    action: "focus-root",
    matches: ({ focusableCount, key }) => key === "Tab" && focusableCount === 0,
  },
  {
    action: "focus-last",
    matches: ({ activeIndex, focusInside, key, shiftKey }) => (
      key === "Tab" && shiftKey && (!focusInside || activeIndex <= 0)
    ),
  },
  {
    action: "focus-first",
    matches: ({ activeIndex, focusInside, focusableCount, key, shiftKey }) => (
      key === "Tab"
      && !shiftKey
      && (!focusInside || activeIndex === focusableCount - 1)
    ),
  },
];

export function resolveDialogKeyboardAction(
  context: DialogKeyboardContext,
): DialogKeyboardAction {
  return DIALOG_KEYBOARD_RULES.find((rule) => rule.matches(context))?.action ?? "ignore";
}
