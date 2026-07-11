import { useCallback } from "react";

import type { LoopCatalogItem } from "@/types/capability/loop";

import type { ComposerDraftController } from "./use-composer-draft";

interface UseComposerGoalActionsOptions {
  closeMention: () => void;
  draft: ComposerDraftController;
  enableLoops: boolean;
  fallbackErrorMessage: string;
  focusTextarea: () => void;
  goalCreateDisabledReason: string | null;
  onCreateGoal?: (objective: string) => Promise<void>;
  onCreateLoopGoal?: (loop: LoopCatalogItem) => Promise<void>;
}

export function useComposerGoalActions({
  closeMention,
  draft,
  enableLoops,
  fallbackErrorMessage,
  focusTextarea,
  goalCreateDisabledReason,
  onCreateGoal,
  onCreateLoopGoal,
}: UseComposerGoalActionsOptions) {
  const {
    applyPrompt,
    cancelGoal,
    resetAfterGoal,
    setActionMenuOpen,
    setGoalCreating,
    setGoalError,
    setLoopPickerOpen,
    startGoal,
    state: { input, isGoalCreating },
  } = draft;
  const canCreateGoal = Boolean(onCreateGoal);
  const canUseLoop = [
    enableLoops,
    [Boolean(onCreateLoopGoal), canCreateGoal].some(Boolean),
  ].every(Boolean);
  const blockedReason = normalizeBlockedReason(goalCreateDisabledReason);

  const submitGoal = useCallback(async () => {
    const objective = input.trim();
    if (
      !objective
      || isGoalCreating
      || !onCreateGoal
      || blockedReason
    ) {
      return;
    }

    setGoalCreating(true);
    setGoalError(null);
    try {
      await onCreateGoal(objective);
      resetAfterGoal();
    } catch (error) {
      setGoalError(
        error instanceof Error ? error.message : fallbackErrorMessage,
      );
    } finally {
      setGoalCreating(false);
    }
  }, [
    blockedReason,
    input,
    isGoalCreating,
    fallbackErrorMessage,
    onCreateGoal,
    resetAfterGoal,
    setGoalCreating,
    setGoalError,
  ]);

  const startGoalInput = useCallback(() => {
    if (!canCreateGoal) {
      return;
    }
    startGoal();
    closeMention();
    focusTextarea();
  }, [canCreateGoal, closeMention, focusTextarea, startGoal]);

  const cancelGoalInput = useCallback(() => {
    cancelGoal();
    focusTextarea();
  }, [cancelGoal, focusTextarea]);

  const toggleGoalInput = useCallback((checked: boolean) => {
    if (checked) {
      startGoalInput();
    } else {
      cancelGoalInput();
    }
  }, [cancelGoalInput, startGoalInput]);

  const openLoopPicker = useCallback(() => {
    if (!canUseLoop) {
      return;
    }
    setActionMenuOpen(false);
    setLoopPickerOpen(true);
  }, [
    canUseLoop,
    setActionMenuOpen,
    setLoopPickerOpen,
  ]);

  const applyLoopPrompt = useCallback((loop: LoopCatalogItem) => {
    applyPrompt(loop.kickoff_prompt, canCreateGoal ? "goal" : "message");
    closeMention();
    focusTextarea();
  }, [applyPrompt, canCreateGoal, closeMention, focusTextarea]);

  const handleLoopSelect = useCallback(async (loop: LoopCatalogItem) => {
    if (!onCreateLoopGoal) {
      applyLoopPrompt(loop);
      return;
    }
    setGoalError(null);
    closeMention();
    await onCreateLoopGoal(loop);
    resetAfterGoal();
  }, [
    applyLoopPrompt,
    closeMention,
    onCreateLoopGoal,
    resetAfterGoal,
    setGoalError,
  ]);

  return {
    blockedReason,
    canCreateGoal,
    canUseLoop,
    cancelGoalInput,
    handleLoopSelect,
    openLoopPicker,
    submitGoal,
    toggleGoalInput,
  };
}

function normalizeBlockedReason(reason: string | null): string | null {
  return reason?.trim() || null;
}
