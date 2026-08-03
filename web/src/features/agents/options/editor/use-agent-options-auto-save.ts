"use client";

import { useEffect, useRef } from "react";

const AGENT_OPTIONS_AUTO_SAVE_DELAY_MS = 500;

interface UseAgentOptionsAutoSaveOptions {
  canSave: boolean;
  draftRevision: number;
  enabled: boolean;
  isDirty: boolean;
  isSaving: boolean;
  save: () => Promise<void>;
  scopeKey: string;
}

interface AutoSaveAttempt {
  draftRevision: number;
  scopeKey: string;
}

type AutoSaveScheduleState = Omit<UseAgentOptionsAutoSaveOptions, "save"> & {
  attempted: AutoSaveAttempt | null;
};

export function useAgentOptionsAutoSave({
  canSave,
  draftRevision,
  enabled,
  isDirty,
  isSaving,
  save,
  scopeKey,
}: UseAgentOptionsAutoSaveOptions): void {
  const attemptedRef = useRef<AutoSaveAttempt | null>(null);

  useEffect(() => {
    if (!shouldScheduleAgentOptionsAutoSave({
      attempted: attemptedRef.current,
      canSave,
      draftRevision,
      enabled,
      isDirty,
      isSaving,
      scopeKey,
    })) {
      return undefined;
    }

    const timer = window.setTimeout(() => {
      attemptedRef.current = { draftRevision, scopeKey };
      void save();
    }, AGENT_OPTIONS_AUTO_SAVE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [canSave, draftRevision, enabled, isDirty, isSaving, save, scopeKey]);
}

export function shouldScheduleAgentOptionsAutoSave({
  attempted,
  canSave,
  draftRevision,
  enabled,
  isDirty,
  isSaving,
  scopeKey,
}: AutoSaveScheduleState): boolean {
  return enabled
    && isDirty
    && canSave
    && !isSaving
    && !(
      attempted?.scopeKey === scopeKey
      && attempted.draftRevision === draftRevision
    );
}
