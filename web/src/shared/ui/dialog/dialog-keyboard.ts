// INPUT: 按键、Shift、焦点位置、可聚焦数量与子浮层打开事实。
// OUTPUT: 关闭、焦点循环或忽略的纯动作判定。
// POS: Dialog 键盘规则真相；不读取 DOM 或执行关闭副作用。
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
