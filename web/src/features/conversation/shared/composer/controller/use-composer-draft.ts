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
  type ComposerGoalConfirmationIdentity,
  type ComposerGoalSubmission,
  useComposerDraftStore,
} from "../composer-draft-store";
import type { ComposerInputMode } from "../composer-model";
import { readObservedComposerGoal } from "../composer-goal-observation";

interface ComposerDraftTransientState {
  isActionMenuOpen: boolean;
  isLoopPickerOpen: boolean;
}

interface ComposerDraftState
  extends ComposerDraftSnapshot, ComposerDraftTransientState {
  goalError: string | null;
  isGoalConfirming: boolean;
  isGoalCreating: boolean;
}

type DraftTransition = (
  state: ComposerDraftTransientState,
) => ComposerDraftTransientState;

const INITIAL_DRAFT_STATE: ComposerDraftTransientState = {
  isActionMenuOpen: false,
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
  beginGoalSubmission: () => ComposerGoalSubmission | null;
  claimMessageSubmission: () => ComposerDraftSnapshot | null;
  completeGoalSubmission: (submission: ComposerGoalSubmission) => boolean;
  failGoalSubmission: (
    submission: ComposerGoalSubmission,
    errorMessage: string,
    confirmationIdentity?: ComposerGoalConfirmationIdentity | null,
  ) => boolean;
  markGoalSubmissionConfirming: (
    submission: ComposerGoalSubmission,
    confirmationIdentity?: ComposerGoalConfirmationIdentity | null,
  ) => boolean;
  restoreFailedMessageSubmission: (
    submittedDraft: ComposerDraftSnapshot,
  ) => boolean;
  setActionMenuOpen: Dispatch<SetStateAction<boolean>>;
  setAttachments: Dispatch<SetStateAction<ComposerLocalAttachment[]>>;
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
  const goalError = useComposerDraftStore(
    (state) => state.goal_error_by_scope[draftScopeKey] ?? null,
  );
  const isGoalCreating = useComposerDraftStore(
    (state) => Boolean(state.goal_submission_by_scope[draftScopeKey]),
  );
  const isGoalConfirming = useComposerDraftStore(
    (state) => (
      state.goal_submission_by_scope[draftScopeKey]?.phase === "confirming"
    ),
  );
  const beginGoalSubmission = useComposerDraftStore(
    (state) => state.begin_goal_submission,
  );
  const claimComposerDraftForSubmission = useComposerDraftStore(
    (state) => state.claim_composer_draft_for_submission,
  );
  const restoreComposerDraftAfterFailedSubmission = useComposerDraftStore(
    (state) => state.restore_composer_draft_after_failed_submission,
  );
  const completeGoalSubmission = useComposerDraftStore(
    (state) => state.complete_goal_submission,
  );
  const failGoalSubmission = useComposerDraftStore(
    (state) => state.fail_goal_submission,
  );
  const markGoalSubmissionConfirming = useComposerDraftStore(
    (state) => state.mark_goal_submission_confirming,
  );
  const setComposerGoalError = useComposerDraftStore(
    (state) => state.set_goal_error,
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
  const setGoalError = useCallback<Dispatch<SetStateAction<string | null>>>((action) => {
    const current = useComposerDraftStore
      .getState()
      .goal_error_by_scope[draftScopeKey] ?? null;
    setComposerGoalError(
      draftScopeKey,
      resolveStateAction(action, current),
    );
  }, [draftScopeKey, setComposerGoalError]);

  const startGoal = useCallback(() => {
    setInputMode("goal");
    setGoalError(null);
    transition((current) => ({ ...current, isActionMenuOpen: false }));
  }, [setGoalError, setInputMode, transition]);
  const cancelGoal = useCallback(() => {
    setInputMode("message");
    setGoalError(null);
    transition((current) => ({ ...current, isActionMenuOpen: false }));
  }, [setGoalError, setInputMode, transition]);
  const applyPrompt = useCallback((prompt: string, mode: ComposerInputMode) => {
    updateComposerDraft(draftScopeKey, (current) => ({
      ...current,
      input: prompt,
      inputMode: mode,
    }));
    setGoalError(null);
  }, [draftScopeKey, setGoalError, updateComposerDraft]);
  const claimDraftSubmission = useCallback(() => (
    claimComposerDraftForSubmission(draftScopeKey, draftSnapshot.revision)
  ), [
    claimComposerDraftForSubmission,
    draftScopeKey,
    draftSnapshot.revision,
  ]);
  const restoreFailedDraftSubmission = useCallback((
    submittedDraft: ComposerDraftSnapshot,
  ) => restoreComposerDraftAfterFailedSubmission(
    draftScopeKey,
    submittedDraft,
  ), [
    draftScopeKey,
    restoreComposerDraftAfterFailedSubmission,
  ]);
  const beginScopedGoalSubmission = useCallback(() => beginGoalSubmission(
    draftScopeKey,
    draftSnapshot.revision,
    readObservedComposerGoal(draftScopeKey),
  ), [beginGoalSubmission, draftScopeKey, draftSnapshot.revision]);

  return {
    state: {
      ...transientState,
      ...draftSnapshot,
      goalError,
      isGoalConfirming,
      isGoalCreating,
    },
    applyPrompt,
    beginGoalSubmission: beginScopedGoalSubmission,
    cancelGoal,
    claimMessageSubmission: claimDraftSubmission,
    completeGoalSubmission,
    failGoalSubmission,
    markGoalSubmissionConfirming,
    restoreFailedMessageSubmission: restoreFailedDraftSubmission,
    setActionMenuOpen,
    setAttachments,
    setGoalError,
    setInput,
    setLoopPickerOpen,
    setSelectedTargetIDs,
    startGoal,
  };
}
