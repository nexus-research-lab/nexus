/**
 * INPUT: Session 草稿作用域与 Composer 输入/模式动作。
 * OUTPUT: 按 Session 隔离完整用户草稿和瞬时 UI 的控制器。
 * POS: Composer 草稿胶囊与瞬时控制状态的唯一编排入口。
 */
import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";

import type { ComposerLocalAttachment } from "../attachments/composer-local-attachment-model";
import {
  EMPTY_COMPOSER_DRAFT,
  type ComposerDraftSnapshot,
  useComposerDraftStore,
} from "../composer-draft-store";
import type { ComposerInputMode } from "../composer-model";

interface ComposerDraftTransientState {
  goalError: string | null;
  isActionMenuOpen: boolean;
  isGoalCreating: boolean;
  isLoopPickerOpen: boolean;
}

interface ComposerDraftState
  extends ComposerDraftSnapshot, ComposerDraftTransientState {}

type DraftTransition = (
  state: ComposerDraftTransientState,
) => ComposerDraftTransientState;

const INITIAL_DRAFT_STATE: ComposerDraftTransientState = {
  goalError: null,
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
  claimMessageSubmission: () => ComposerDraftSnapshot | null;
  resetAfterGoal: () => void;
  restoreFailedMessageSubmission: (
    submittedDraft: ComposerDraftSnapshot,
  ) => boolean;
  setActionMenuOpen: Dispatch<SetStateAction<boolean>>;
  setAttachments: Dispatch<SetStateAction<ComposerLocalAttachment[]>>;
  setGoalCreating: Dispatch<SetStateAction<boolean>>;
  setGoalError: Dispatch<SetStateAction<string | null>>;
  setInput: Dispatch<SetStateAction<string>>;
  setLoopPickerOpen: Dispatch<SetStateAction<boolean>>;
  setSelectedTargetIDs: Dispatch<SetStateAction<string[]>>;
  startGoal: () => void;
}

export function useComposerDraft(
  draftScopeKey: string,
): ComposerDraftController {
  const draftSnapshot = useComposerDraftStore(
    (state) => state.drafts_by_scope[draftScopeKey] ?? EMPTY_COMPOSER_DRAFT,
  );
  const claimComposerDraftForSubmission = useComposerDraftStore(
    (state) => state.claim_composer_draft_for_submission,
  );
  const restoreComposerDraftAfterFailedSubmission = useComposerDraftStore(
    (state) => state.restore_composer_draft_after_failed_submission,
  );
  const updateComposerDraft = useComposerDraftStore(
    (state) => state.update_composer_draft,
  );
  const [transientState, setTransientState] = useResettableState(
    INITIAL_DRAFT_STATE,
    draftScopeKey,
  );
  const transition = useCallback((apply: DraftTransition) => {
    setTransientState((current) => apply(current));
  }, [setTransientState]);

  const setAttachments = useCallback<
    Dispatch<SetStateAction<ComposerLocalAttachment[]>>
  >((action) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      attachments: resolveStateAction(action, current.attachments),
    }));
  }, [draftScopeKey, updateComposerDraft]);
  const setInput = useCallback<Dispatch<SetStateAction<string>>>((action) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      input: resolveStateAction(action, current.input),
    }));
  }, [draftScopeKey, updateComposerDraft]);
  const setInputMode = useCallback((inputMode: ComposerInputMode) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      inputMode,
    }));
  }, [draftScopeKey, updateComposerDraft]);
  const setSelectedTargetIDs = useCallback<
    Dispatch<SetStateAction<string[]>>
  >((action) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      selectedTargetIDs: resolveStateAction(
        action,
        current.selectedTargetIDs,
      ),
    }));
  }, [draftScopeKey, updateComposerDraft]);
  const setActionMenuOpen = useCallback<Dispatch<SetStateAction<boolean>>>((action) => {
    transition((current) => ({
      ...current,
      isActionMenuOpen: resolveStateAction(action, current.isActionMenuOpen),
    }));
  }, [transition]);
  const setLoopPickerOpen = useCallback<Dispatch<SetStateAction<boolean>>>((action) => {
    transition((current) => ({
      ...current,
      isLoopPickerOpen: resolveStateAction(action, current.isLoopPickerOpen),
    }));
  }, [transition]);
  const setGoalCreating = useCallback<Dispatch<SetStateAction<boolean>>>((action) => {
    transition((current) => ({
      ...current,
      isGoalCreating: resolveStateAction(action, current.isGoalCreating),
    }));
  }, [transition]);
  const setGoalError = useCallback<Dispatch<SetStateAction<string | null>>>((action) => {
    transition((current) => ({
      ...current,
      goalError: resolveStateAction(action, current.goalError),
    }));
  }, [transition]);

  const startGoal = useCallback(() => {
    setInputMode("goal");
    transition((current) => ({
      ...current,
      goalError: null,
      isActionMenuOpen: false,
    }));
  }, [setInputMode, transition]);
  const cancelGoal = useCallback(() => {
    setInputMode("message");
    transition((current) => ({
      ...current,
      goalError: null,
      isActionMenuOpen: false,
    }));
  }, [setInputMode, transition]);
  const applyPrompt = useCallback((prompt: string, mode: ComposerInputMode) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      input: prompt,
      inputMode: mode,
    }));
    transition((current) => ({
      ...current,
      goalError: null,
    }));
  }, [draftScopeKey, transition, updateComposerDraft]);
  const claimMessageSubmission = useCallback(() => (
    claimComposerDraftForSubmission(draftScopeKey, draftSnapshot.revision)
  ), [
    claimComposerDraftForSubmission,
    draftScopeKey,
    draftSnapshot.revision,
  ]);
  const restoreFailedMessageSubmission = useCallback((
    submittedDraft: ComposerDraftSnapshot,
  ) => restoreComposerDraftAfterFailedSubmission(
    draftScopeKey,
    submittedDraft,
  ), [
    draftScopeKey,
    restoreComposerDraftAfterFailedSubmission,
  ]);
  const resetAfterGoal = useCallback(() => {
    const cleared = claimComposerDraftForSubmission(
      draftScopeKey,
      draftSnapshot.revision,
    );
    if (!cleared) {
      return;
    }
    transition((current) => ({
      ...current,
      goalError: null,
    }));
  }, [
    claimComposerDraftForSubmission,
    draftScopeKey,
    draftSnapshot.revision,
    transition,
  ]);

  return {
    state: {
      ...transientState,
      ...draftSnapshot,
    },
    applyPrompt,
    cancelGoal,
    claimMessageSubmission,
    resetAfterGoal,
    restoreFailedMessageSubmission,
    setActionMenuOpen,
    setAttachments,
    setGoalCreating,
    setGoalError,
    setInput,
    setLoopPickerOpen,
    setSelectedTargetIDs,
    startGoal,
  };
}
