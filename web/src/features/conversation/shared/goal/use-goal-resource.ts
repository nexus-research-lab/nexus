"use client";

import { useCallback, useRef, useState } from "react";

import { getCurrentGoalApi } from "@/lib/api/conversation/goal-api";
import { ApiRequestError } from "@/lib/api/core/http";
import type { Goal } from "@/types/conversation/goal";

export type GoalCommandPhase = "clearing" | "pausing" | "resuming" | "updating";

interface GoalSnapshot {
  available: boolean;
  error: string | null;
  goal: Goal | null;
  loading: boolean;
  sessionKey: string | null;
}

interface ActiveGoalCommand {
  id: number;
  phase: GoalCommandPhase;
  sessionKey: string;
}

interface GoalCommandOutcome {
  goal: Goal | null;
  ok: boolean;
}

interface GoalResourceOptions {
  onGoalResolved: (goal: Goal | null) => void;
  sessionKey: string | null;
}

function emptySnapshot(sessionKey: string | null): GoalSnapshot {
  return {
    available: true,
    error: null,
    goal: null,
    loading: Boolean(sessionKey),
    sessionKey,
  };
}

function requestError(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function hasStatus(error: unknown, status: number): boolean {
  return error instanceof ApiRequestError && error.status === status;
}

export function useGoalResource({
  onGoalResolved,
  sessionKey,
}: GoalResourceOptions) {
  const [snapshot, setSnapshot] = useState<GoalSnapshot>(() => (
    emptySnapshot(sessionKey)
  ));
  const [command, setCommand] = useState<ActiveGoalCommand | null>(null);
  const requestVersionRef = useRef(0);
  const commandSequenceRef = useRef(0);
  const activeCommandRef = useRef<ActiveGoalCommand | null>(null);
  const deferredRefreshRef = useRef<string | null>(null);
  const currentSessionKeyRef = useRef(sessionKey);
  currentSessionKeyRef.current = sessionKey;

  const visibleSnapshot = snapshot.sessionKey === sessionKey
    ? snapshot
    : emptySnapshot(sessionKey);
  const goal = visibleSnapshot.goal;
  const currentCommand = command?.sessionKey === sessionKey ? command : null;

  const refresh = useCallback(async () => {
    if (!sessionKey) {
      requestVersionRef.current += 1;
      setSnapshot(emptySnapshot(null));
      return;
    }
    if (activeCommandRef.current?.sessionKey === sessionKey) {
      deferredRefreshRef.current = sessionKey;
      return;
    }

    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;
    setSnapshot((current) => ({
      available: true,
      error: null,
      goal: current.sessionKey === sessionKey ? current.goal : null,
      loading: true,
      sessionKey,
    }));
    try {
      const current = await getCurrentGoalApi(sessionKey);
      if (requestVersionRef.current !== requestVersion) {
        return;
      }
      setSnapshot({
        available: true,
        error: null,
        goal: current,
        loading: false,
        sessionKey,
      });
      onGoalResolved(current);
    } catch (error) {
      if (requestVersionRef.current !== requestVersion) {
        return;
      }
      if (hasStatus(error, 403) || hasStatus(error, 404)) {
        const available = !hasStatus(error, 403);
        setSnapshot({
          available,
          error: null,
          goal: null,
          loading: false,
          sessionKey,
        });
        onGoalResolved(null);
        return;
      }
      setSnapshot((current) => current.sessionKey === sessionKey
        ? {
            ...current,
            error: requestError(error, "Goal 状态读取失败"),
            loading: false,
          }
        : current);
    }
  }, [onGoalResolved, sessionKey]);

  const runCommand = useCallback(async (
    phase: GoalCommandPhase,
    action: (goalId: string) => Promise<Goal | null>,
    fallbackError: string,
  ): Promise<GoalCommandOutcome> => {
    if (!goal || !sessionKey || activeCommandRef.current) {
      return { goal: null, ok: false };
    }

    const nextCommand: ActiveGoalCommand = {
      id: commandSequenceRef.current + 1,
      phase,
      sessionKey,
    };
    commandSequenceRef.current = nextCommand.id;
    activeCommandRef.current = nextCommand;
    setCommand(nextCommand);
    const operationVersion = requestVersionRef.current + 1;
    requestVersionRef.current = operationVersion;
    setSnapshot((current) => current.sessionKey === sessionKey
      ? { ...current, error: null }
      : current);

    try {
      const updated = await action(goal.id);
      if (requestVersionRef.current !== operationVersion) {
        return { goal: null, ok: false };
      }
      setSnapshot({
        available: true,
        error: null,
        goal: updated,
        loading: false,
        sessionKey,
      });
      onGoalResolved(updated);
      return { goal: updated, ok: true };
    } catch (error) {
      if (requestVersionRef.current === operationVersion) {
        setSnapshot((current) => current.sessionKey === sessionKey
          ? { ...current, error: requestError(error, fallbackError) }
          : current);
      }
      return { goal: null, ok: false };
    } finally {
      if (activeCommandRef.current?.id === nextCommand.id) {
        activeCommandRef.current = null;
      }
      setCommand((current) => current?.id === nextCommand.id ? null : current);
      if (deferredRefreshRef.current === sessionKey) {
        deferredRefreshRef.current = null;
        if (currentSessionKeyRef.current === sessionKey) {
          void refresh();
        }
      }
    }
  }, [goal, onGoalResolved, refresh, sessionKey]);

  return {
    available: visibleSnapshot.available,
    error: visibleSnapshot.error,
    goal,
    isLoading: visibleSnapshot.loading || currentCommand !== null,
    phase: currentCommand?.phase ?? null,
    refresh,
    runCommand,
  };
}
