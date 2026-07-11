"use client";

import { useCallback, useRef, useState } from "react";

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
import type { useAgentNameValidation } from "./use-agent-name-validation";
import type { useAgentSaveFeedback } from "./use-agent-save-feedback";

interface SaveToken {
  draftKey: string;
  id: number;
  scopeKey: string;
}

interface SaveCommandLabels {
  failed: string;
  success: string;
}

interface UseAgentOptionsSaveCommandOptions {
  draft: AgentOptionsDraft;
  feedback: ReturnType<typeof useAgentSaveFeedback>;
  hasTitleChanged: boolean;
  labels: SaveCommandLabels;
  mode: AgentOptionsMode;
  onSave: AgentOptionsFormProps["onSave"];
  onSaveSuccess?: () => void;
  onValidateName?: AgentOptionsFormProps["onValidateName"];
  scopeKey: string;
  sourceOptions: AgentEditorInitialOptions;
  validation: ReturnType<typeof useAgentNameValidation>;
}

interface SaveTransactionContext {
  draft: AgentOptionsDraft;
  draftKeyRef: { current: string };
  expected: SaveToken;
  hasTitleChanged: boolean;
  mode: AgentOptionsMode;
  onSave: AgentOptionsFormProps["onSave"];
  onValidateName?: AgentOptionsFormProps["onValidateName"];
  saveTokenRef: { current: SaveToken | null };
  scopeKeyRef: { current: string };
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
  draft,
  feedback,
  hasTitleChanged,
  labels,
  mode,
  onSave,
  onSaveSuccess,
  onValidateName,
  scopeKey,
  sourceOptions,
  validation,
}: UseAgentOptionsSaveCommandOptions) {
  const title = draft.title.trim();
  const draftKey = JSON.stringify(draft);
  const draftKeyRef = useRef(draftKey);
  draftKeyRef.current = draftKey;
  const scopeKeyRef = useRef(scopeKey);
  scopeKeyRef.current = scopeKey;
  const saveSequenceRef = useRef(0);
  const saveTokenRef = useRef<SaveToken | null>(null);
  const [savingScopeKey, setSavingScopeKey] = useState<string | null>(null);
  const isSaving = savingScopeKey === scopeKey;
  const canSave = [
    Boolean(title),
    !validation.isValidating,
    !isInvalidNameValidation(validation.result),
    !isSaving,
  ].every(Boolean);

  const save = useCallback(async () => {
    if (!canStartSave(canSave, saveTokenRef.current, scopeKey)) {
      return;
    }
    const token = createSaveToken(saveSequenceRef, draftKey, scopeKey);
    saveTokenRef.current = token;
    setSavingScopeKey(scopeKey);
    feedback.clear();

    try {
      await runSaveTransaction({
        draft,
        draftKeyRef,
        expected: token,
        hasTitleChanged,
        mode,
        onSave,
        onValidateName,
        saveTokenRef,
        scopeKeyRef,
        sourceOptions,
        title,
        validation,
      });
      reportSaveSuccess(onSaveSuccess, feedback, labels.success);
    } catch (error) {
      handleSaveFailure({
        error,
        expected: token,
        fallbackError: labels.failed,
        feedback,
        saveTokenRef,
        scopeKeyRef,
        draftKeyRef,
      });
    } finally {
      finishSave(token, saveTokenRef, setSavingScopeKey);
    }
  }, [
    canSave,
    draft,
    draftKey,
    feedback,
    hasTitleChanged,
    labels.failed,
    labels.success,
    mode,
    onSave,
    onSaveSuccess,
    onValidateName,
    scopeKey,
    sourceOptions,
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
  current: SaveToken | null,
  scopeKey: string,
): boolean {
  return [enabled, current?.scopeKey !== scopeKey].every(Boolean);
}

function createSaveToken(
  sequenceRef: { current: number },
  draftKey: string,
  scopeKey: string,
): SaveToken {
  const token = { draftKey, id: sequenceRef.current + 1, scopeKey };
  sequenceRef.current = token.id;
  return token;
}

async function runSaveTransaction(context: SaveTransactionContext): Promise<void> {
  const validation = await resolveNameValidation(context);
  assertCurrentSave(context);
  assertNameAccepted(validation);
  const submission = buildAgentOptionsSubmission(context.draft, context.sourceOptions);
  await context.onSave(submission.title, submission.options, submission.identity);
  assertCurrentSave(context);
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

function assertCurrentSave(context: SaveTransactionContext): void {
  if (!isCurrentSave(
    context.expected,
    context.saveTokenRef.current,
    context.scopeKeyRef,
    context.draftKeyRef,
  )) {
    throw SAVE_ABORT.stale;
  }
}

function assertNameAccepted(validation: ValidationOutcome): void {
  if (validation.required && isInvalidNameValidation(validation.result)) {
    throw SAVE_ABORT.invalidName;
  }
}

function handleSaveFailure({
  draftKeyRef,
  error,
  expected,
  fallbackError,
  feedback,
  saveTokenRef,
  scopeKeyRef,
}: {
  draftKeyRef: { current: string };
  error: unknown;
  expected: SaveToken;
  fallbackError: string;
  feedback: ReturnType<typeof useAgentSaveFeedback>;
  saveTokenRef: { current: SaveToken | null };
  scopeKeyRef: { current: string };
}): void {
  if (SAVE_ABORTS.has(error)) {
    return;
  }
  if (!isCurrentSave(expected, saveTokenRef.current, scopeKeyRef, draftKeyRef)) {
    return;
  }
  feedback.showError(resolveSaveErrorMessage(error, fallbackError));
}

function resolveSaveErrorMessage(error: unknown, fallbackError: string): string {
  return error instanceof Error ? error.message : fallbackError;
}

function finishSave(
  expected: SaveToken,
  saveTokenRef: { current: SaveToken | null },
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
  expected: SaveToken,
  current: SaveToken | null,
  currentScopeKey: { current: string },
  currentDraftKey: { current: string },
): boolean {
  return current?.id === expected.id
    && currentScopeKey.current === expected.scopeKey
    && currentDraftKey.current === expected.draftKey;
}
