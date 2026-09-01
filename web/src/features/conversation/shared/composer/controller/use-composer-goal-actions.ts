// INPUT: Goal 草稿提交、exact transport correlation 与服务端失败证据。
// OUTPUT: 明确未应用的草稿恢复，或 accepted/committed/unknown 的确认中状态。
// POS: Composer Goal mutation 边界；普通 Error 正文不可见，未知结果不可重复提交。
import { useCallback } from "react";

import {
  getErrorMessage,
  projectMutationFailure,
} from "@/lib/error-message";
import type { LoopCatalogItem } from "@/types/capability/loop";

import type { ComposerGoalConfirmationIdentity } from "../composer-draft-store";
import type { ComposerDraftController } from "./use-composer-draft";

interface UseComposerGoalActionsOptions {
  closeMention: () => void;
  draft: ComposerDraftController;
  enableLoops: boolean;
  fallbackErrorMessage: string;
  failureImpact: string;
  failureNextStep: string;
  focusTextarea: () => void;
  goalCreateDisabledReason: string | null;
  onCreateGoal?: (objective: string) => Promise<void>;
  onCreateLoopGoal?: (loop: LoopCatalogItem) => Promise<void>;
}

function isRequestAcceptanceUnknown(error: unknown): error is Error {
  return error instanceof Error
    && error.name === "RequestAcceptanceUnknownError";
}

function isRequestAcceptanceRejected(error: unknown): error is Error {
  return error instanceof Error
    && error.name === "RequestAcceptanceRejectedError";
}

function requiresGoalReconciliation(error: unknown): boolean {
  if (isRequestAcceptanceUnknown(error)) {
    return true;
  }
  if (isRequestAcceptanceRejected(error)) {
    return false;
  }
  const failure = projectMutationFailure(error, "Goal 请求结果待确认");
  return failure.effect !== "not_applied"
    && (
      failure.category !== null
      || failure.code !== null
      || failure.transportRequestId !== null
    );
}

function buildGoalFailureMessage(
  error: unknown,
  fallback: string,
  impact: string,
  nextStep: string,
): string {
  return [
    getErrorMessage(error, fallback),
    impact,
    nextStep,
  ].join(" ");
}

function readConfirmationIdentity(
  error: unknown,
): ComposerGoalConfirmationIdentity | null {
  if (!(error instanceof Error) || !("correlation" in error)) {
    return null;
  }
  const correlation = error.correlation;
  if (!correlation || typeof correlation !== "object") {
    return null;
  }
  const candidate = correlation as Partial<ComposerGoalConfirmationIdentity>;
  if (
    typeof candidate.clientMessageId !== "string"
    || typeof candidate.clientRequestId !== "string"
    || typeof candidate.sessionKey !== "string"
  ) {
    return null;
  }
  return {
    clientMessageId: candidate.clientMessageId,
    clientRequestId: candidate.clientRequestId,
    sessionKey: candidate.sessionKey,
  };
}

export function useComposerGoalActions({
  closeMention,
  draft,
  enableLoops,
  fallbackErrorMessage,
  failureImpact,
  failureNextStep,
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
    markGoalSubmissionConfirming,
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
        buildGoalFailureMessage(
          error,
          fallbackErrorMessage,
          failureImpact,
          failureNextStep,
        ),
        readConfirmationIdentity(error),
      );
      return;
    }
    setGoalError(null);
    try {
      await createPromise;
      completeGoalSubmission(submission);
    } catch (error) {
      if (requiresGoalReconciliation(error)) {
        // ACK 未知或服务端只证明 accepted/committed：保留原 scope 的互斥
        // 提交状态，等待 durable Goal 读取对账，不能伪成功或恢复草稿。
        markGoalSubmissionConfirming(
          submission,
          readConfirmationIdentity(error),
        );
        return;
      }
      failGoalSubmission(
        submission,
        buildGoalFailureMessage(
          error,
          fallbackErrorMessage,
          failureImpact,
          failureNextStep,
        ),
        readConfirmationIdentity(error),
      );
    }
  }, [
    blockedReason,
    input,
    isGoalCreating,
    fallbackErrorMessage,
    failureImpact,
    failureNextStep,
    onCreateGoal,
    beginGoalSubmission,
    completeGoalSubmission,
    failGoalSubmission,
    markGoalSubmissionConfirming,
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
        buildGoalFailureMessage(
          error,
          fallbackErrorMessage,
          failureImpact,
          failureNextStep,
        ),
        readConfirmationIdentity(error),
      );
      throw error;
    }
    try {
      await createPromise;
      completeGoalSubmission(submission);
    } catch (error) {
      if (requiresGoalReconciliation(error)) {
        markGoalSubmissionConfirming(
          submission,
          readConfirmationIdentity(error),
        );
        return;
      }
      failGoalSubmission(
        submission,
        buildGoalFailureMessage(
          error,
          fallbackErrorMessage,
          failureImpact,
          failureNextStep,
        ),
        readConfirmationIdentity(error),
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
    failureImpact,
    failureNextStep,
    onCreateLoopGoal,
    markGoalSubmissionConfirming,
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
