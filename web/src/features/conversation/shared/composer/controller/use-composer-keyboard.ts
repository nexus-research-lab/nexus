import { useCallback, useRef } from "react";
import type { KeyboardEvent } from "react";

import {
  COMPOSITION_END_ENTER_GUARD_MS,
  MENTION_NAVIGATION_KEYS,
  type ComposerNativeKeyboardEvent,
  isCaretOnFirstLine,
  isCaretOnLastLine,
  isImeKeyboardEvent,
} from "../composer-model";

interface UseComposerKeyboardOptions {
  historyIndex: number;
  historyItemCount: number;
  isLoading: boolean;
  mentionActive: boolean;
  onSend: () => void | Promise<void>;
  onStop?: () => void;
  recallNext: () => void;
  recallPrevious: () => void;
}

interface KeyboardCommand {
  matches: boolean;
  run: () => void;
}

export function useComposerKeyboard({
  historyIndex,
  historyItemCount,
  isLoading,
  mentionActive,
  onSend,
  onStop,
  recallNext,
  recallPrevious,
}: UseComposerKeyboardOptions) {
  const isComposingRef = useRef(false);
  const ignoreNextEnterRef = useRef(false);
  const lastCompositionEndAtRef = useRef(0);

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
    ignoreNextEnterRef.current = false;
  }, []);

  const handleCompositionEnd = useCallback((timeStamp: number) => {
    isComposingRef.current = false;
    ignoreNextEnterRef.current = true;
    lastCompositionEndAtRef.current = timeStamp;
  }, []);

  const handleKeyDown = useCallback((
    event: KeyboardEvent<HTMLTextAreaElement>,
  ) => {
    const nativeEvent = event.nativeEvent as ComposerNativeKeyboardEvent;

    // Safari 可能在中文候选词确认后补发一个不带 composing 标记的 Enter。
    if (isComposingRef.current || isImeKeyboardEvent(nativeEvent)) {
      return;
    }

    if (event.key === "Enter" && ignoreNextEnterRef.current) {
      const withinGuard = lastCompositionEndAtRef.current > 0
        && event.timeStamp - lastCompositionEndAtRef.current
          <= COMPOSITION_END_ENTER_GUARD_MS;
      ignoreNextEnterRef.current = false;
      if (withinGuard) {
        return;
      }
    } else if (event.key !== "Enter") {
      ignoreNextEnterRef.current = false;
    }

    if (mentionActive && MENTION_NAVIGATION_KEYS.has(event.key)) {
      return;
    }

    const commands: KeyboardCommand[] = [
      {
        matches: event.key === "Enter" && !event.shiftKey,
        run: () => void onSend(),
      },
      {
        matches: event.key === "ArrowUp"
          && historyItemCount > 0
          && (event.ctrlKey || isCaretOnFirstLine(event.currentTarget)),
        run: recallPrevious,
      },
      {
        matches: event.key === "ArrowDown"
          && historyIndex >= 0
          && (event.ctrlKey || isCaretOnLastLine(event.currentTarget)),
        run: recallNext,
      },
      {
        matches: event.key === "Escape" && isLoading && Boolean(onStop),
        run: () => onStop?.(),
      },
    ];
    const command = commands.find((item) => item.matches);
    if (command) {
      event.preventDefault();
      command.run();
    }
  }, [
    historyIndex,
    historyItemCount,
    isLoading,
    mentionActive,
    onSend,
    onStop,
    recallNext,
    recallPrevious,
  ]);

  return {
    handleCompositionEnd,
    handleCompositionStart,
    handleKeyDown,
  };
}
