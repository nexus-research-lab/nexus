export type OperationStageWindowKeyboardAction =
  | "cycle_next"
  | "cycle_previous"
  | "focus"
  | "close"
  | "minimize"
  | "zoom";

export interface OperationStageWindowKeyboardInput {
  key: string;
  metaKey?: boolean;
  ctrlKey?: boolean;
  altKey?: boolean;
  shiftKey?: boolean;
}

export function resolveOperationWindowKeyboardAction(
  input: OperationStageWindowKeyboardInput,
): OperationStageWindowKeyboardAction | null {
  if (input.altKey) {
    return null;
  }
  if (input.metaKey && !input.ctrlKey && is_cycle_window_key(input.key)) {
    return input.shiftKey || input.key === "~" ? "cycle_previous" : "cycle_next";
  }
  if (input.shiftKey) {
    return null;
  }
  if (!input.metaKey && !input.ctrlKey) {
    if (input.key === "Enter" || input.key === " ") {
      return "focus";
    }
    if (input.key === "Escape") {
      return "minimize";
    }
    return null;
  }
  if (input.metaKey && !input.ctrlKey) {
    const key = input.key.toLowerCase();
    if (key === "w") {
      return "close";
    }
    if (key === "m") {
      return "minimize";
    }
    if (input.key === "Enter") {
      return "zoom";
    }
  }
  if (input.metaKey && input.ctrlKey && input.key.toLowerCase() === "f") {
    return "zoom";
  }
  return null;
}

export function shouldHandleStageDesktopKeyboardAction(
  action: OperationStageWindowKeyboardAction,
): boolean {
  return action !== "focus";
}

function is_cycle_window_key(key: string): boolean {
  return key === "`" || key === "~" || key === "Backquote";
}
