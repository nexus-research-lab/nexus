import {
  useCallback,
  useMemo,
  useRef,
  type MutableRefObject,
} from "react";
import type { KeyboardEvent } from "react";

import {
  MENTION_NAVIGATION_KEYS,
  type ComposerNativeKeyboardEvent,
  isCaretOnFirstLine,
  isCaretOnLastLine,
  isImeKeyboardEvent,
  isWithinCompositionEndEnterGuard,
} from "../composer-model";

interface UseComposerKeyboardOptions {
  historyIndex: number;
  historyItemCount: number;
  isLoading: boolean;
  mentionActive: boolean;
  onSend: () => void | Promise<void>;
  onStop: () => void;
  recallNext: () => void;
  recallPrevious: () => void;
}

interface CompositionState {
  ignoreNextEnterRef: MutableRefObject<boolean>;
  isComposingRef: MutableRefObject<boolean>;
  lastCompositionEndAtRef: MutableRefObject<number>;
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
  const compositionState = useMemo<CompositionState>(() => ({
    ignoreNextEnterRef,
    isComposingRef,
    lastCompositionEndAtRef,
  }), []);

  const handleCompositionStart = useCallback(() => {
    compositionState.isComposingRef.current = true;
    compositionState.ignoreNextEnterRef.current = false;
  }, [compositionState]);

  const handleCompositionEnd = useCallback((timeStamp: number) => {
    compositionState.isComposingRef.current = false;
    compositionState.ignoreNextEnterRef.current = true;
    compositionState.lastCompositionEndAtRef.current = timeStamp;
  }, [compositionState]);

  const handleKeyDown = useCallback((
    event: KeyboardEvent<HTMLTextAreaElement>,
  ) => {
    if (shouldIgnoreKeyboardEvent(
      event,
      compositionState,
      mentionActive,
    )) {
      return;
    }
    const command = resolveKeyboardCommand(event, {
      historyIndex,
      historyItemCount,
      isLoading,
      mentionActive,
      onSend,
      onStop,
      recallNext,
      recallPrevious,
    });
    if (!command) {
      return;
    }
    event.preventDefault();
    command();
  }, [
    compositionState,
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

function shouldIgnoreKeyboardEvent(
  event: KeyboardEvent<HTMLTextAreaElement>,
  compositionState: CompositionState,
  mentionActive: boolean,
): boolean {
  // Safari 可能在中文候选词确认后补发一个不带 composing 标记的 Enter。
  if (isCompositionEvent(event, compositionState)) {
    return true;
  }
  if (consumeCompositionEnterGuard(event, compositionState)) {
    event.preventDefault();
    return true;
  }
  return isMentionNavigationEvent(event, mentionActive);
}

function isCompositionEvent(
  event: KeyboardEvent<HTMLTextAreaElement>,
  compositionState: CompositionState,
): boolean {
  return [
    compositionState.isComposingRef.current,
    isImeKeyboardEvent(event.nativeEvent as ComposerNativeKeyboardEvent),
  ].some(Boolean);
}

function consumeCompositionEnterGuard(
  event: KeyboardEvent<HTMLTextAreaElement>,
  compositionState: CompositionState,
): boolean {
  if (event.key !== "Enter") {
    compositionState.ignoreNextEnterRef.current = false;
    return false;
  }
  if (!compositionState.ignoreNextEnterRef.current) {
    return false;
  }
  compositionState.ignoreNextEnterRef.current = false;
  return isWithinCompositionEndEnterGuard(
    event.timeStamp,
    compositionState.lastCompositionEndAtRef.current,
  );
}

function isMentionNavigationEvent(
  event: KeyboardEvent<HTMLTextAreaElement>,
  mentionActive: boolean,
): boolean {
  return [mentionActive, MENTION_NAVIGATION_KEYS.has(event.key)].every(Boolean);
}

function resolveKeyboardCommand(
  event: KeyboardEvent<HTMLTextAreaElement>,
  options: UseComposerKeyboardOptions,
): (() => void) | null {
  const commands: KeyboardCommand[] = [
    {
      matches: [event.key === "Enter", !event.shiftKey].every(Boolean),
      run: () => void options.onSend(),
    },
    {
      matches: [
        event.key === "ArrowUp",
        options.historyItemCount > 0,
        [event.ctrlKey, isCaretOnFirstLine(event.currentTarget)].some(Boolean),
      ].every(Boolean),
      run: options.recallPrevious,
    },
    {
      matches: [
        event.key === "ArrowDown",
        options.historyIndex >= 0,
        [event.ctrlKey, isCaretOnLastLine(event.currentTarget)].some(Boolean),
      ].every(Boolean),
      run: options.recallNext,
    },
    {
      matches: [
        event.key === "Escape",
        options.isLoading,
      ].every(Boolean),
      run: options.onStop,
    },
  ];
  return commands.find((command) => command.matches)?.run ?? null;
}
