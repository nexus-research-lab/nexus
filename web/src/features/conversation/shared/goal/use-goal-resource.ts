"use client";

import { useCallback, useRef, useState } from "react";

import { getCurrentGoalApi } from "@/lib/api/conversation/goal-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { getErrorMessage } from "@/lib/error-message";
import type { Goal } from "@/types/conversation/goal";

import type { GoalCommandPhase } from "./goal-model";

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

interface GoalCommandTransaction {
  command: ActiveGoalCommand;
  goalId: string;
  operationVersion: number;
}

interface GoalResourceOptions {
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

function hasStatus(error: unknown, status: number): boolean {
  return error instanceof ApiRequestError && error.status === status;
}

export function useGoalResource({
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
        return;
      }
      setSnapshot((current) => current.sessionKey === sessionKey
        ? {
            ...current,
            error: getErrorMessage(error, "Goal 状态读取失败"),
            loading: false,
          }
        : current);
    }
  }, [sessionKey]);

  const beginCommand = useCallback((
    phase: GoalCommandPhase,
  ): GoalCommandTransaction | null => {
    if (!goal || !sessionKey || activeCommandRef.current) {
      return null;
    }

    const nextCommand: ActiveGoalCommand = {
      id: commandSequenceRef.current + 1,
      phase,
      sessionKey,
    };
    const operationVersion = requestVersionRef.current + 1;
    commandSequenceRef.current = nextCommand.id;
    requestVersionRef.current = operationVersion;
    activeCommandRef.current = nextCommand;
    setCommand(nextCommand);
    setSnapshot((current) => current.sessionKey === sessionKey
      ? { ...current, error: null }
      : current);
    return {
      command: nextCommand,
      goalId: goal.id,
      operationVersion,
    };
  }, [goal, sessionKey]);

  const resolveCommand = useCallback((
    transaction: GoalCommandTransaction,
    updated: Goal | null,
  ): boolean => {
    if (requestVersionRef.current !== transaction.operationVersion) {
      return false;
    }
    setSnapshot({
      available: true,
      error: null,
      goal: updated,
      loading: false,
      sessionKey: transaction.command.sessionKey,
    });
    return true;
  }, []);

  const rejectCommand = useCallback((
    transaction: GoalCommandTransaction,
    error: unknown,
    fallbackError: string,
  ) => {
    if (requestVersionRef.current !== transaction.operationVersion) {
      return;
    }
    setSnapshot((current) => (
      current.sessionKey === transaction.command.sessionKey
        ? { ...current, error: getErrorMessage(error, fallbackError) }
        : current
    ));
  }, []);

  const finishCommand = useCallback((transaction: GoalCommandTransaction) => {
    const finishedCommand = transaction.command;
    if (activeCommandRef.current?.id === finishedCommand.id) {
      activeCommandRef.current = null;
    }
    setCommand((current) => current?.id === finishedCommand.id ? null : current);
    if (deferredRefreshRef.current !== finishedCommand.sessionKey) {
      return;
    }
    deferredRefreshRef.current = null;
    if (currentSessionKeyRef.current === finishedCommand.sessionKey) {
      void refresh();
    }
  }, [refresh]);

  const runCommand = useCallback(async (
    phase: GoalCommandPhase,
    action: (goalId: string) => Promise<Goal | null>,
    fallbackError: string,
  ): Promise<GoalCommandOutcome> => {
    const transaction = beginCommand(phase);
    if (!transaction) {
      return { goal: null, ok: false };
    }

    try {
      const updated = await action(transaction.goalId);
      if (!resolveCommand(transaction, updated)) {
        return { goal: null, ok: false };
      }
      return { goal: updated, ok: true };
    } catch (error) {
      rejectCommand(transaction, error, fallbackError);
      return { goal: null, ok: false };
    } finally {
      finishCommand(transaction);
    }
  }, [beginCommand, finishCommand, rejectCommand, resolveCommand]);

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
