// INPUT: The current Composer textarea and its native value/selection.
// OUTPUT: Caret-line queries and explicit end-of-draft focus/scroll restoration.
// POS: Composer DOM adapter; domain decisions remain in composer-model and no commands are dispatched here.

export function focusComposerInputAtEnd(
  target: HTMLTextAreaElement,
): void {
  const caretPosition = target.value.length;
  target.focus({ preventScroll: true });
  target.setSelectionRange(caretPosition, caretPosition);
  target.scrollTop = target.scrollHeight;
}

export function isCaretOnFirstLine(target: HTMLTextAreaElement): boolean {
  const { end, start } = readSelectionRange(target);
  return [
    start === end,
    !target.value.slice(0, start).includes("\n"),
  ].every(Boolean);
}

export function isCaretOnLastLine(target: HTMLTextAreaElement): boolean {
  const { end, start } = readSelectionRange(target);
  return [
    start === end,
    !target.value.slice(end).includes("\n"),
  ].every(Boolean);
}

function readSelectionRange(target: HTMLTextAreaElement) {
  return {
    end: target.selectionEnd ?? 0,
    start: target.selectionStart ?? 0,
  };
}
