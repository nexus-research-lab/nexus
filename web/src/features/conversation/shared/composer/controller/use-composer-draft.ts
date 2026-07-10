import { useCallback, useReducer } from "react";
import type { Dispatch, SetStateAction } from "react";

import type { ComposerInputMode } from "../composer-model";

interface ComposerDraftState {
  goalError: string | null;
  input: string;
  inputMode: ComposerInputMode;
  isActionMenuOpen: boolean;
  isGoalCreating: boolean;
  isLoopPickerOpen: boolean;
}

type DraftTransition = (state: ComposerDraftState) => ComposerDraftState;

const INITIAL_DRAFT_STATE: ComposerDraftState = {
  goalError: null,
  input: "",
  inputMode: "message",
  isActionMenuOpen: false,
  isGoalCreating: false,
  isLoopPickerOpen: false,
};

function resolveStateAction<T>(action: SetStateAction<T>, current: T): T {
  return typeof action === "function"
    ? (action as (value: T) => T)(current)
    : action;
}

export interface ComposerDraftController {
  state: ComposerDraftState;
  applyPrompt: (prompt: string, mode: ComposerInputMode) => void;
  cancelGoal: () => void;
  resetAfterGoal: () => void;
  setActionMenuOpen: Dispatch<SetStateAction<boolean>>;
  setGoalCreating: Dispatch<SetStateAction<boolean>>;
  setGoalError: Dispatch<SetStateAction<string | null>>;
  setInput: Dispatch<SetStateAction<string>>;
  setLoopPickerOpen: Dispatch<SetStateAction<boolean>>;
  startGoal: () => void;
}

export function useComposerDraft(): ComposerDraftController {
  const [state, transition] = useReducer(
    (current: ComposerDraftState, apply: DraftTransition) => apply(current),
    INITIAL_DRAFT_STATE,
  );

  const setInput = useCallback<Dispatch<SetStateAction<string>>>((action) => {
    transition((current) => ({
      ...current,
      input: resolveStateAction(action, current.input),
    }));
  }, []);
  const setActionMenuOpen = useCallback<Dispatch<SetStateAction<boolean>>>((action) => {
    transition((current) => ({
      ...current,
      isActionMenuOpen: resolveStateAction(action, current.isActionMenuOpen),
    }));
  }, []);
  const setLoopPickerOpen = useCallback<Dispatch<SetStateAction<boolean>>>((action) => {
    transition((current) => ({
      ...current,
      isLoopPickerOpen: resolveStateAction(action, current.isLoopPickerOpen),
    }));
  }, []);
  const setGoalCreating = useCallback<Dispatch<SetStateAction<boolean>>>((action) => {
    transition((current) => ({
      ...current,
      isGoalCreating: resolveStateAction(action, current.isGoalCreating),
    }));
  }, []);
  const setGoalError = useCallback<Dispatch<SetStateAction<string | null>>>((action) => {
    transition((current) => ({
      ...current,
      goalError: resolveStateAction(action, current.goalError),
    }));
  }, []);

  const startGoal = useCallback(() => {
    transition((current) => ({
      ...current,
      goalError: null,
      inputMode: "goal",
      isActionMenuOpen: false,
    }));
  }, []);
  const cancelGoal = useCallback(() => {
    transition((current) => ({
      ...current,
      goalError: null,
      inputMode: "message",
      isActionMenuOpen: false,
    }));
  }, []);
  const applyPrompt = useCallback((prompt: string, mode: ComposerInputMode) => {
    transition((current) => ({
      ...current,
      goalError: null,
      input: prompt,
      inputMode: mode,
    }));
  }, []);
  const resetAfterGoal = useCallback(() => {
    transition((current) => ({
      ...current,
      goalError: null,
      input: "",
      inputMode: "message",
    }));
  }, []);

  return {
    state,
    applyPrompt,
    cancelGoal,
    resetAfterGoal,
    setActionMenuOpen,
    setGoalCreating,
    setGoalError,
    setInput,
    setLoopPickerOpen,
    startGoal,
  };
}
