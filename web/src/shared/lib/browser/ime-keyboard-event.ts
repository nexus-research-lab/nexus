// INPUT: Browser keyboard-event composition flags and legacy IME key codes.
// OUTPUT: Whether the event belongs to text composition rather than an application shortcut.
// POS: Neutral keyboard boundary shared by Composer and message editing; owns no submit command or composition timing state.

export function isImeKeyboardEvent(event: {
  isComposing?: boolean;
  key?: string;
  keyCode?: number;
  which?: number;
}): boolean {
  return Boolean(event.isComposing || event.key === "Process" || event.keyCode === 229 || event.which === 229);
}
