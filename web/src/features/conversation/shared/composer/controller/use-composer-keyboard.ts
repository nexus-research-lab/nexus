// INPUT: Native textarea events, composition lifecycle and Composer navigation/submit actions.
// OUTPUT: Ordered composition, Safari Enter, Slash and Mention guards before keyboard commands.
// POS: Composer keyboard controller; neutral IME event detection belongs to shared/lib/browser.

import {
  useCallback,
  useMemo,
  useRef,
  type MutableRefObject,
} from "react";
import type { KeyboardEvent } from "react";
import { isCaretOnFirstLine, isCaretOnLastLine } from "./composer-textarea";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";

import {
  MENTION_NAVIGATION_KEYS,
  isWithinCompositionEndEnterGuard,
} from "../composer-model";

interface UseComposerKeyboardOptions {
  historyIndex: number;
  historyItemCount: number;
  isLoading: boolean;
  mentionActive: boolean;
  onSlashCommandKeyDown: (
    event: KeyboardEvent<HTMLTextAreaElement>,
  ) => boolean;
  onSend: () => void | Promise<void>;
  onStop?: () => void;
  recallNext: () => void;
  recallPrevious: () => void;
  slashCommandActive: boolean;
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

type StandardKeyboardCommandOptions = Pick<
  UseComposerKeyboardOptions,
  | "historyIndex"
  | "historyItemCount"
  | "isLoading"
  | "onSend"
  | "onStop"
  | "recallNext"
  | "recallPrevious"
>;

export function useComposerKeyboard({
  historyIndex,
  historyItemCount,
  isLoading,
  mentionActive,
  onSlashCommandKeyDown,
  onSend,
  onStop,
  recallNext,
  recallPrevious,
  slashCommandActive,
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
    if (shouldIgnoreCompositionEvent(event, compositionState)) {
      return;
    }
    if (slashCommandActive && onSlashCommandKeyDown(event)) {
      return;
    }
    if (isMentionNavigationEvent(event, mentionActive)) {
      return;
    }
    const command = resolveKeyboardCommand(event, {
      historyIndex,
      historyItemCount,
      isLoading,
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
    onSlashCommandKeyDown,
    onSend,
    onStop,
    recallNext,
    recallPrevious,
    slashCommandActive,
  ]);

  return {
    handleCompositionEnd,
    handleCompositionStart,
    handleKeyDown,
  };
}

function shouldIgnoreCompositionEvent(
  event: KeyboardEvent<HTMLTextAreaElement>,
  compositionState: CompositionState,
): boolean {
  // Safari 可能在中文候选词确认后补发一个不带 composing 标记的 Enter。
  if (isCompositionEvent(event, compositionState)) {
    return true;
  }
  if (consumeCompositionEnterGuard(event, compositionState)) {
    event.preventDefault();
    return true;
  }
  return false;
}

function isCompositionEvent(
  event: KeyboardEvent<HTMLTextAreaElement>,
  compositionState: CompositionState,
): boolean {
  return [
    compositionState.isComposingRef.current,
    isImeKeyboardEvent(event.nativeEvent),
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
  options: StandardKeyboardCommandOptions,
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
  ];
  if (options.onStop) {
    commands.push({
      matches: [
        event.key === "Escape",
        options.isLoading,
      ].every(Boolean),
      run: options.onStop,
    });
  }
  return commands.find((command) => command.matches)?.run ?? null;
}
