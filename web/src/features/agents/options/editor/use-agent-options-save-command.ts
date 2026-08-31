"use client";

import { useCallback, useRef, useState } from "react";

import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import type {
  AgentEditorInitialOptions,
  AgentOptionsFormProps,
  AgentOptionsMode,
} from "../agent-options-editor-model";
import {
  buildAgentOptionsSubmission,
  type AgentOptionsDraft,
} from "./agent-options-draft";
import {
  isAgentOptionsSaveCurrent,
  type AgentOptionsSaveToken,
} from "./agent-options-save-transaction";
import type { useAgentNameValidation } from "./use-agent-name-validation";
import type { useAgentSaveFeedback } from "./use-agent-save-feedback";

interface SaveCommandLabels {
  failed: string;
  failures: Record<MutationFailureEffect, SaveFailureCopy>;
  preferCopyMessage?: boolean;
  success: string;
}

interface SaveFailureCopy {
  impact: string;
  message: string;
  nextStep: string;
}

interface UseAgentOptionsSaveCommandOptions {
  commandScopeKey: string;
  draft: AgentOptionsDraft;
  draftRevisionRef: { current: number };
  feedback: ReturnType<typeof useAgentSaveFeedback>;
  hasTitleChanged: boolean;
  labels: SaveCommandLabels;
  mode: AgentOptionsMode;
  onSave: AgentOptionsFormProps["onSave"];
  onSaveSuccess?: () => void;
  onValidateName?: AgentOptionsFormProps["onValidateName"];
  sourceScopeKey: string;
  sourceOptions: AgentEditorInitialOptions;
  validation: ReturnType<typeof useAgentNameValidation>;
}

interface SaveTransactionContext {
  commandScopeKeyRef: { current: string };
  draft: AgentOptionsDraft;
  draftRevisionRef: { current: number };
  expected: AgentOptionsSaveToken;
  hasTitleChanged: boolean;
  mode: AgentOptionsMode;
  onSave: AgentOptionsFormProps["onSave"];
  onValidateName?: AgentOptionsFormProps["onValidateName"];
  saveTokenRef: { current: AgentOptionsSaveToken | null };
  sourceScopeKeyRef: { current: string };
  sourceOptions: AgentEditorInitialOptions;
  title: string;
  validation: ReturnType<typeof useAgentNameValidation>;
}

interface ValidationOutcome {
  required: boolean;
  result: AgentNameValidationResult | null;
}

// 作用域切换和名称拒绝会终止当前事务，但不应伪装成保存失败。
const SAVE_ABORT = {
  invalidName: Symbol("invalid-agent-name"),
  stale: Symbol("stale-agent-options-save"),
} as const;
const SAVE_ABORTS = new Set<unknown>(Object.values(SAVE_ABORT));

export function useAgentOptionsSaveCommand({
  commandScopeKey,
  draft,
  draftRevisionRef,
  feedback,
  hasTitleChanged,
  labels,
  mode,
  onSave,
  onSaveSuccess,
  onValidateName,
  sourceScopeKey,
  sourceOptions,
  validation,
}: UseAgentOptionsSaveCommandOptions) {
  const title = draft.title.trim();
  const commandScopeKeyRef = useRef(commandScopeKey);
  commandScopeKeyRef.current = commandScopeKey;
  const sourceScopeKeyRef = useRef(sourceScopeKey);
  sourceScopeKeyRef.current = sourceScopeKey;
  const saveSequenceRef = useRef(0);
  const saveTokenRef = useRef<AgentOptionsSaveToken | null>(null);
  const [savingScopeKey, setSavingScopeKey] = useState<string | null>(null);
  const [repeatBlocked, setRepeatBlocked] = useResettableState(false, sourceScopeKey);
  const isSaving = savingScopeKey === commandScopeKey;
  const canSave = [
    Boolean(title),
    !validation.isValidating,
    !isInvalidNameValidation(validation.result),
    !isSaving,
    !repeatBlocked,
  ].every(Boolean);

  const save = useCallback(async () => {
    if (!canStartSave(canSave, saveTokenRef.current, commandScopeKey)) {
      return;
    }
    const token = createSaveToken(
      saveSequenceRef,
      commandScopeKey,
      draftRevisionRef.current,
      sourceScopeKey,
    );
    saveTokenRef.current = token;
    setSavingScopeKey(commandScopeKey);
    feedback.clear();

    try {
      await runSaveTransaction({
        commandScopeKeyRef,
        draft,
        draftRevisionRef,
        expected: token,
        hasTitleChanged,
        mode,
        onSave,
        onValidateName,
        saveTokenRef,
        sourceScopeKeyRef,
        sourceOptions,
        title,
        validation,
      });
      reportSaveSuccess(onSaveSuccess, feedback, labels.success);
    } catch (error) {
      const failure = handleSaveFailure({
        error,
        expected: token,
        fallbackError: labels.failed,
        failureCopies: labels.failures,
        preferCopyMessage: labels.preferCopyMessage === true,
        feedback,
        saveTokenRef,
        commandScopeKeyRef,
        draftRevisionRef,
        sourceScopeKeyRef,
      });
      if (failure.blocksRepeat) {
        setRepeatBlocked(true);
      }
    } finally {
      finishSave(token, saveTokenRef, setSavingScopeKey);
    }
  }, [
    canSave,
    commandScopeKey,
    draft,
    draftRevisionRef,
    feedback,
    hasTitleChanged,
    labels.failed,
    labels.failures,
    labels.success,
    labels.preferCopyMessage,
    mode,
    onSave,
    onSaveSuccess,
    onValidateName,
    sourceScopeKey,
    sourceOptions,
    setRepeatBlocked,
    title,
    validation,
  ]);

  return { canSave, isSaving, save };
}

function reportSaveSuccess(
  onSaveSuccess: (() => void) | undefined,
  feedback: ReturnType<typeof useAgentSaveFeedback>,
  successMessage: string,
): void {
  const report = onSaveSuccess ?? (() => feedback.showSuccess(successMessage));
  report();
}

function canStartSave(
  enabled: boolean,
  current: AgentOptionsSaveToken | null,
  commandScopeKey: string,
): boolean {
  return [enabled, current?.commandScopeKey !== commandScopeKey].every(Boolean);
}

function createSaveToken(
  sequenceRef: { current: number },
  commandScopeKey: string,
  draftRevision: number,
  sourceScopeKey: string,
): AgentOptionsSaveToken {
  const token = {
    commandScopeKey,
    draftRevision,
    id: sequenceRef.current + 1,
    sourceScopeKey,
  };
  sequenceRef.current = token.id;
  return token;
}

async function runSaveTransaction(context: SaveTransactionContext): Promise<void> {
  const validation = await resolveNameValidation(context);
  assertCurrentSave(context, true);
  assertNameAccepted(validation);
  const submission = buildAgentOptionsSubmission(context.draft, context.sourceOptions);
  await context.onSave(submission.title, submission.options, submission.identity);
  assertCurrentSave(context, false);
}

async function resolveNameValidation({
  hasTitleChanged,
  mode,
  onValidateName,
  title,
  validation,
}: SaveTransactionContext): Promise<ValidationOutcome> {
  const required = Boolean(onValidateName)
    && [mode === "create", hasTitleChanged].some(Boolean);
  if (!required) {
    return { required, result: validation.result };
  }
  const result = await selectValidationResult(validation, title);
  return { required, result };
}

async function selectValidationResult(
  validation: ReturnType<typeof useAgentNameValidation>,
  title: string,
): Promise<AgentNameValidationResult | null> {
  if (validation.result?.name === title) {
    return validation.result;
  }
  return validation.validateNow(title);
}

function assertCurrentSave(
  context: SaveTransactionContext,
  requireSourceScope: boolean,
): void {
  if (!isCurrentSave(context, requireSourceScope)) {
    throw SAVE_ABORT.stale;
  }
}

function assertNameAccepted(validation: ValidationOutcome): void {
  if (validation.required && isInvalidNameValidation(validation.result)) {
    throw SAVE_ABORT.invalidName;
  }
}

function handleSaveFailure({
  commandScopeKeyRef,
  draftRevisionRef,
  error,
  expected,
  fallbackError,
  failureCopies,
  preferCopyMessage,
  feedback,
  saveTokenRef,
  sourceScopeKeyRef,
}: {
  commandScopeKeyRef: { current: string };
  draftRevisionRef: { current: number };
  error: unknown;
  expected: AgentOptionsSaveToken;
  fallbackError: string;
  failureCopies: Record<MutationFailureEffect, SaveFailureCopy>;
  preferCopyMessage: boolean;
  feedback: ReturnType<typeof useAgentSaveFeedback>;
  saveTokenRef: { current: AgentOptionsSaveToken | null };
  sourceScopeKeyRef: { current: string };
}): { blocksRepeat: boolean } {
  if (SAVE_ABORTS.has(error)) {
    return { blocksRepeat: false };
  }
  if (!isAgentOptionsSaveCurrent(expected, {
    commandScopeKey: commandScopeKeyRef.current,
    draftRevision: draftRevisionRef.current,
    sourceScopeKey: sourceScopeKeyRef.current,
    token: saveTokenRef.current,
  }, false)) {
    return { blocksRepeat: false };
  }
  const failure = projectMutationFailure(error, fallbackError);
  const copy = failureCopies[failure.effect];
  const blocksRepeat = failure.effect !== "not_applied"
    && sourceScopeKeyRef.current === expected.sourceScopeKey;
  feedback.showFailure({
    ...copy,
    blocksRepeat,
    message: failure.effect === "not_applied" && !preferCopyMessage
      ? failure.message
      : copy.message,
    tone: failure.effect === "accepted" || failure.effect === "committed"
      ? "warning"
      : "error",
  });
  return { blocksRepeat };
}

function finishSave(
  expected: AgentOptionsSaveToken,
  saveTokenRef: { current: AgentOptionsSaveToken | null },
  setSavingScopeKey: (scopeKey: string | null) => void,
): void {
  if (saveTokenRef.current?.id !== expected.id) {
    return;
  }
  saveTokenRef.current = null;
  setSavingScopeKey(null);
}

function isInvalidNameValidation(
  result: AgentNameValidationResult | null,
): boolean {
  return Boolean(result && (!result.is_valid || !result.is_available));
}

function isCurrentSave(
  context: SaveTransactionContext,
  requireSourceScope: boolean,
): boolean {
  return isAgentOptionsSaveCurrent(context.expected, {
    commandScopeKey: context.commandScopeKeyRef.current,
    draftRevision: context.draftRevisionRef.current,
    sourceScopeKey: context.sourceScopeKeyRef.current,
    token: context.saveTokenRef.current,
  }, requireSourceScope);
}
