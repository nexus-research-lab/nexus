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

function isRequestAcceptanceUnknown(error: unknown): boolean {
  return error instanceof Error
    && error.name === "RequestAcceptanceUnknownError";
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
    beginGoalSubmission,
    cancelGoal,
    completeGoalSubmission,
    failGoalSubmission,
    setActionMenuOpen,
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

    const submission = beginGoalSubmission();
    if (!submission) {
      return;
    }

    let createPromise: Promise<void>;
    try {
      createPromise = onCreateGoal(objective);
    } catch (error) {
      failGoalSubmission(
        submission,
        error instanceof Error ? error.message : fallbackErrorMessage,
      );
      return;
    }
    setGoalError(null);
    try {
      await createPromise;
      completeGoalSubmission(submission);
    } catch (error) {
      if (isRequestAcceptanceUnknown(error)) {
        // 超时或组件卸载只能说明 ACK 未知，不能把可能已创建的 Goal
        // 重新塞回草稿，诱导用户重复提交。
        completeGoalSubmission(submission);
        return;
      }
      failGoalSubmission(
        submission,
        error instanceof Error ? error.message : fallbackErrorMessage,
      );
    }
  }, [
    blockedReason,
    input,
    isGoalCreating,
    fallbackErrorMessage,
    onCreateGoal,
    beginGoalSubmission,
    completeGoalSubmission,
    failGoalSubmission,
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
    const submission = beginGoalSubmission();
    if (!submission) {
      return;
    }
    let createPromise: Promise<void>;
    try {
      createPromise = onCreateLoopGoal(loop);
    } catch (error) {
      failGoalSubmission(
        submission,
        error instanceof Error ? error.message : fallbackErrorMessage,
      );
      throw error;
    }
    try {
      await createPromise;
      completeGoalSubmission(submission);
    } catch (error) {
      if (isRequestAcceptanceUnknown(error)) {
        completeGoalSubmission(submission);
        return;
      }
      failGoalSubmission(
        submission,
        error instanceof Error ? error.message : fallbackErrorMessage,
      );
      throw error;
    }
  }, [
    applyLoopPrompt,
    beginGoalSubmission,
    closeMention,
    completeGoalSubmission,
    failGoalSubmission,
    fallbackErrorMessage,
    onCreateLoopGoal,
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
