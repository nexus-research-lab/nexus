"use client";

/**
 * INPUT: exact Session key, auth owner generation, owner-scoped Goal reads and one current lifecycle mutation intent.
 * OUTPUT: scope-fenced Goal/binding snapshot, conservative unknown-result lock and read-only reconciliation.
 * POS: Goal panel resource owner; it never replays a mutation or infers rejection from an unchanged read.
 */

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import {
  getCurrentGoalApi,
  getGoalExecutionBindingApi,
} from "@/lib/api/conversation/goal-api";
import {
  ApiRequestError,
  UnauthorizedError,
} from "@/lib/api/core/http-error";
import {
  getResourceFailure,
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import type {
  Goal,
  GoalExecutionBinding,
} from "@/types/conversation/goal";

import {
  createGoalLifecycleIntent,
  reconcileGoalLifecycleIntent,
  type GoalLifecycleIntent,
  type GoalLifecycleMutationInput,
  type GoalLifecycleOperation,
  type GoalMutationBlockReason,
  type GoalReliabilityState,
} from "./goal-lifecycle-recovery";
import type { GoalCommandPhase } from "./goal-model";

interface GoalSnapshot {
  binding: GoalExecutionBinding | null;
  goal: Goal | null;
  loading: boolean;
  ownerScopeGeneration: number;
  sessionKey: string | null;
}

interface ActiveGoalCommand {
  id: number;
  ownerScopeGeneration: number;
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
  intent: GoalLifecycleIntent;
  operationVersion: number;
}

interface GoalMutationLock {
  detail: string;
  effect: Exclude<MutationFailureEffect, "not_applied">;
  intent: GoalLifecycleIntent;
  ownerScopeGeneration: number;
}

interface DeferredGoalRefresh {
  ownerScopeGeneration: number;
  sessionKey: string;
}

interface ScopedGoalReliabilityState extends GoalReliabilityState {
  blocksMutations: boolean;
  ownerScopeGeneration: number;
}

interface GoalResourceOptions {
  sessionKey: string | null;
}

type GoalReadResult =
  | {
      binding: GoalExecutionBinding | null;
      goal: Goal | null;
      ok: true;
    }
  | {
      error: unknown;
      goal: Goal | null;
      ok: false;
      stage: "binding" | "goal" | "scope";
    };

interface RefreshReason {
  committedOperation?: GoalLifecycleOperation;
}

function emptySnapshot(
  sessionKey: string | null,
  ownerScopeGeneration: number,
): GoalSnapshot {
  return {
    binding: null,
    goal: null,
    loading: Boolean(sessionKey),
    ownerScopeGeneration,
    sessionKey,
  };
}

function commandPhase(operation: GoalLifecycleOperation): GoalCommandPhase {
  switch (operation) {
    case "clear":
      return "clearing";
    case "pause":
      return "pausing";
    case "resume":
      return "resuming";
    case "update":
      return "updating";
  }
}

function hasStatus(error: unknown, status: number): boolean {
  return (
    error instanceof ApiRequestError || error instanceof UnauthorizedError
  ) && error.status === status;
}

async function readGoalSnapshot(sessionKey: string): Promise<GoalReadResult> {
  let goal: Goal | null;
  try {
    goal = await getCurrentGoalApi(sessionKey);
  } catch (error) {
    if (hasStatus(error, 404)) {
      return { binding: null, goal: null, ok: true };
    }
    return { error, goal: null, ok: false, stage: "goal" };
  }
  if (!goal) {
    return { binding: null, goal: null, ok: true };
  }
  if (goal.session_key !== sessionKey) {
    return {
      error: new Error("读取到的 Goal 不属于当前会话"),
      goal: null,
      ok: false,
      stage: "scope",
    };
  }
  try {
    const binding = await getGoalExecutionBindingApi(goal.id);
    return { binding, goal, ok: true };
  } catch (error) {
    if (hasStatus(error, 404)) {
      return { binding: null, goal: null, ok: true };
    }
    return { error, goal, ok: false, stage: "binding" };
  }
}

function reliabilityState(
  input: Omit<ScopedGoalReliabilityState, "access"> & {
    access?: ScopedGoalReliabilityState["access"];
  },
): ScopedGoalReliabilityState {
  return { access: input.access ?? null, ...input };
}

function validGoalCommandResult(
  transaction: GoalCommandTransaction,
  updated: Goal | null,
): boolean {
  if (transaction.intent.operation === "clear") {
    return updated === null;
  }
  return updated?.id === transaction.goalId
    && updated.session_key === transaction.command.sessionKey;
}

export function useGoalResource({
  sessionKey,
}: GoalResourceOptions) {
  const ownerScopeGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const [snapshot, setSnapshot] = useState<GoalSnapshot>(() => (
    emptySnapshot(sessionKey, ownerScopeGeneration)
  ));
  const [command, setCommand] = useState<ActiveGoalCommand | null>(null);
  const [mutationLock, setMutationLock] = useState<GoalMutationLock | null>(null);
  const [reconciling, setReconciling] = useState(false);
  const [reliability, setReliability] = useState<ScopedGoalReliabilityState | null>(null);
  const requestVersionRef = useRef(0);
  const commandSequenceRef = useRef(0);
  const activeCommandRef = useRef<ActiveGoalCommand | null>(null);
  const mutationLockRef = useRef<GoalMutationLock | null>(null);
  const reconcilingRef = useRef(false);
  const deferredRefreshRef = useRef<DeferredGoalRefresh | null>(null);
  const currentSessionKeyRef = useRef(sessionKey);
  const currentOwnerScopeGenerationRef = useRef(ownerScopeGeneration);
  const previousSessionKeyRef = useRef(sessionKey);
  const previousOwnerScopeGenerationRef = useRef(ownerScopeGeneration);
  currentSessionKeyRef.current = sessionKey;
  currentOwnerScopeGenerationRef.current = ownerScopeGeneration;

  const visibleSnapshot = snapshot.sessionKey === sessionKey
    && snapshot.ownerScopeGeneration === ownerScopeGeneration
    ? snapshot
    : emptySnapshot(sessionKey, ownerScopeGeneration);
  const goal = visibleSnapshot.goal;
  const executionBinding = visibleSnapshot.binding;
  const currentCommand = command?.sessionKey === sessionKey
    && command.ownerScopeGeneration === ownerScopeGeneration
    ? command
    : null;
  const currentMutationLock = mutationLock?.intent.sessionKey === sessionKey
    && mutationLock.ownerScopeGeneration === ownerScopeGeneration
    ? mutationLock
    : null;
  const currentReliability = reliability?.sessionKey === sessionKey
    && reliability.ownerScopeGeneration === ownerScopeGeneration
    ? reliability
    : null;

  const updateMutationLock = useCallback((next: GoalMutationLock | null) => {
    mutationLockRef.current = next;
    setMutationLock(next);
  }, []);

  const refreshResource = useCallback(async (
    reason: RefreshReason = {},
  ): Promise<boolean> => {
    const expectedSessionKey = sessionKey;
    const expectedOwnerScopeGeneration = ownerScopeGeneration;
    if (!expectedSessionKey) {
      requestVersionRef.current += 1;
      setSnapshot(emptySnapshot(null, expectedOwnerScopeGeneration));
      setReliability(null);
      return true;
    }
    if (
      activeCommandRef.current?.sessionKey === expectedSessionKey
      && activeCommandRef.current.ownerScopeGeneration
        === expectedOwnerScopeGeneration
    ) {
      deferredRefreshRef.current = {
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
      };
      return false;
    }

    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;
    setSnapshot((current) => ({
      binding: current.sessionKey === expectedSessionKey
          && current.ownerScopeGeneration === expectedOwnerScopeGeneration
        ? current.binding
        : null,
      goal: current.sessionKey === expectedSessionKey
          && current.ownerScopeGeneration === expectedOwnerScopeGeneration
        ? current.goal
        : null,
      loading: true,
      ownerScopeGeneration: expectedOwnerScopeGeneration,
      sessionKey: expectedSessionKey,
    }));

    const result = await readGoalSnapshot(expectedSessionKey);
    if (
      requestVersionRef.current !== requestVersion
      || currentSessionKeyRef.current !== expectedSessionKey
      || currentOwnerScopeGenerationRef.current !== expectedOwnerScopeGeneration
      || !isAuthOwnerScopeGenerationCurrent(expectedOwnerScopeGeneration)
    ) {
      return false;
    }
    if (result.ok) {
      setSnapshot({
        binding: result.binding,
        goal: result.goal,
        loading: false,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
      });
      setReliability(null);
      return true;
    }

    const failure = getResourceFailure(
      result.error,
      result.stage === "goal"
        ? "Goal 状态读取失败"
        : result.stage === "scope"
          ? "Goal 会话范围核对失败"
          : "Goal 与工作图绑定状态读取失败",
    );
    if (failure.access || result.stage === "scope") {
      setSnapshot({
        binding: null,
        goal: null,
        loading: false,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
      });
      setReliability(reliabilityState({
        access: failure.access,
        blocksMutations: true,
        detail: failure.message,
        kind: reason.committedOperation
          ? "mutation_committed_refresh_failed"
          : "access_lost",
        operation: reason.committedOperation ?? null,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
        stale: false,
      }));
      return false;
    }

    setSnapshot((previous) => ({
      binding: null,
      goal: result.stage === "binding"
        ? result.goal
        : previous.sessionKey === expectedSessionKey
            && previous.ownerScopeGeneration === expectedOwnerScopeGeneration
          ? previous.goal
          : null,
      loading: false,
      ownerScopeGeneration: expectedOwnerScopeGeneration,
      sessionKey: expectedSessionKey,
    }));
    setReliability(reliabilityState({
      blocksMutations: result.stage !== "binding",
      detail: failure.message,
      kind: reason.committedOperation
        ? "mutation_committed_refresh_failed"
        : result.stage === "binding" ? "binding_failed" : "read_failed",
      operation: reason.committedOperation ?? null,
      ownerScopeGeneration: expectedOwnerScopeGeneration,
      sessionKey: expectedSessionKey,
      stale: true,
    }));
    return false;
  }, [ownerScopeGeneration, sessionKey]);

  const reconcileLockedMutation = useCallback(async (): Promise<void> => {
    const locked = mutationLockRef.current;
    if (
      !locked
      || currentSessionKeyRef.current !== locked.intent.sessionKey
      || currentOwnerScopeGenerationRef.current !== locked.ownerScopeGeneration
      || !isAuthOwnerScopeGenerationCurrent(locked.ownerScopeGeneration)
      || reconcilingRef.current
    ) {
      return;
    }
    const expectedSessionKey = locked.intent.sessionKey;
    const expectedOwnerScopeGeneration = locked.ownerScopeGeneration;
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;
    reconcilingRef.current = true;
    setReconciling(true);

    const result = await readGoalSnapshot(expectedSessionKey);
    if (
      requestVersionRef.current !== requestVersion
      || currentSessionKeyRef.current !== expectedSessionKey
      || currentOwnerScopeGenerationRef.current !== expectedOwnerScopeGeneration
      || !isAuthOwnerScopeGenerationCurrent(expectedOwnerScopeGeneration)
      || mutationLockRef.current !== locked
    ) {
      if (mutationLockRef.current === locked) {
        reconcilingRef.current = false;
        setReconciling(false);
      }
      return;
    }
    reconcilingRef.current = false;
    setReconciling(false);

    if (!result.ok && result.stage !== "binding") {
      const scopeMismatch = result.stage === "scope";
      const failure = getResourceFailure(
        result.error,
        scopeMismatch ? "Goal 会话范围核对失败" : "Goal 最新状态核对失败",
      );
      if (failure.access || scopeMismatch) {
        if (locked.effect === "committed") {
          if (!scopeMismatch) {
            updateMutationLock(null);
          }
        }
        setSnapshot({
          binding: null,
          goal: null,
          loading: false,
          ownerScopeGeneration: expectedOwnerScopeGeneration,
          sessionKey: expectedSessionKey,
        });
        setReliability(reliabilityState({
          access: failure.access,
          blocksMutations: true,
          detail: failure.message,
          kind: locked.effect === "committed"
            ? "mutation_committed_refresh_failed"
            : "access_lost",
          operation: locked.intent.operation,
          ownerScopeGeneration: expectedOwnerScopeGeneration,
          sessionKey: expectedSessionKey,
          stale: false,
        }));
        return;
      }
      setSnapshot((previous) => ({
        binding: null,
        goal: previous.sessionKey === expectedSessionKey
            && previous.ownerScopeGeneration === expectedOwnerScopeGeneration
          ? previous.goal
          : null,
        loading: false,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
      }));
      setReliability(reliabilityState({
        blocksMutations: true,
        detail: failure.message,
        kind: locked.effect === "committed"
          ? "mutation_committed_refresh_failed"
          : "mutation_reconcile_failed",
        operation: locked.intent.operation,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
        stale: true,
      }));
      return;
    }

    const currentGoal = result.goal;
    const bindingFailure = !result.ok
      ? getResourceFailure(result.error, "Goal 与工作图绑定状态读取失败")
      : null;
    if (bindingFailure?.access) {
      if (locked.effect === "committed") {
        updateMutationLock(null);
      }
      setSnapshot({
        binding: null,
        goal: null,
        loading: false,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
      });
      setReliability(reliabilityState({
        access: bindingFailure.access,
        blocksMutations: true,
        detail: bindingFailure.message,
        kind: locked.effect === "committed"
          ? "mutation_committed_refresh_failed"
          : "access_lost",
        operation: locked.intent.operation,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
        stale: false,
      }));
      return;
    }

    setSnapshot({
      binding: result.ok ? result.binding : null,
      goal: currentGoal,
      loading: false,
      ownerScopeGeneration: expectedOwnerScopeGeneration,
      sessionKey: expectedSessionKey,
    });

    if (locked.effect === "committed") {
      updateMutationLock(null);
      setReliability(reliabilityState({
        blocksMutations: false,
        detail: result.ok ? locked.detail : bindingFailure?.message ?? locked.detail,
        kind: result.ok
          ? "mutation_committed"
          : "mutation_committed_refresh_failed",
        operation: locked.intent.operation,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
        stale: !result.ok,
      }));
      return;
    }

    const outcome = reconcileGoalLifecycleIntent(locked.intent, currentGoal);
    if (outcome === "applied" || outcome === "target_not_current") {
      updateMutationLock(null);
      setReliability(reliabilityState({
        blocksMutations: false,
        detail: result.ok ? locked.detail : bindingFailure?.message ?? locked.detail,
        kind: outcome === "applied"
          ? "mutation_applied"
          : "mutation_target_not_current",
        operation: locked.intent.operation,
        ownerScopeGeneration: expectedOwnerScopeGeneration,
        sessionKey: expectedSessionKey,
        stale: !result.ok,
      }));
      return;
    }
    setReliability(reliabilityState({
      blocksMutations: true,
      detail: result.ok ? locked.detail : bindingFailure?.message ?? locked.detail,
      kind: "mutation_unproven",
      operation: locked.intent.operation,
      ownerScopeGeneration: expectedOwnerScopeGeneration,
      sessionKey: expectedSessionKey,
      stale: !result.ok,
    }));
  }, [updateMutationLock]);

  const refresh = useCallback(async (): Promise<void> => {
    if (
      mutationLockRef.current?.intent.sessionKey === sessionKey
      && mutationLockRef.current.ownerScopeGeneration === ownerScopeGeneration
    ) {
      await reconcileLockedMutation();
      return;
    }
    await refreshResource();
  }, [ownerScopeGeneration, reconcileLockedMutation, refreshResource, sessionKey]);

  const beginCommand = useCallback((
    input: GoalLifecycleMutationInput,
  ): GoalCommandTransaction | null => {
    if (
      !goal
      || !sessionKey
      || goal.session_key !== sessionKey
      || activeCommandRef.current
      || (
        mutationLockRef.current?.intent.sessionKey === sessionKey
        && mutationLockRef.current.ownerScopeGeneration === ownerScopeGeneration
      )
    ) {
      return null;
    }

    const nextCommand: ActiveGoalCommand = {
      id: commandSequenceRef.current + 1,
      ownerScopeGeneration,
      phase: commandPhase(input.operation),
      sessionKey,
    };
    const operationVersion = requestVersionRef.current + 1;
    commandSequenceRef.current = nextCommand.id;
    requestVersionRef.current = operationVersion;
    activeCommandRef.current = nextCommand;
    setCommand(nextCommand);
    setReliability(null);
    return {
      command: nextCommand,
      goalId: goal.id,
      intent: createGoalLifecycleIntent(goal, sessionKey, input),
      operationVersion,
    };
  }, [goal, ownerScopeGeneration, sessionKey]);

  const resolveCommand = useCallback((
    transaction: GoalCommandTransaction,
    updated: Goal | null,
  ): boolean => {
    if (
      requestVersionRef.current !== transaction.operationVersion
      || currentSessionKeyRef.current !== transaction.command.sessionKey
      || currentOwnerScopeGenerationRef.current
        !== transaction.command.ownerScopeGeneration
      || !isAuthOwnerScopeGenerationCurrent(
        transaction.command.ownerScopeGeneration,
      )
    ) {
      return false;
    }
    setSnapshot((current) => ({
      binding: updated
          && current.ownerScopeGeneration
            === transaction.command.ownerScopeGeneration
          && current.goal?.id === updated.id
        ? current.binding
        : null,
      goal: updated,
      loading: false,
      ownerScopeGeneration: transaction.command.ownerScopeGeneration,
      sessionKey: transaction.command.sessionKey,
    }));
    setReliability(null);
    return true;
  }, []);

  const rejectCommand = useCallback((
    transaction: GoalCommandTransaction,
    error: unknown,
  ): boolean => {
    if (
      requestVersionRef.current !== transaction.operationVersion
      || currentSessionKeyRef.current !== transaction.command.sessionKey
      || currentOwnerScopeGenerationRef.current
        !== transaction.command.ownerScopeGeneration
      || !isAuthOwnerScopeGenerationCurrent(
        transaction.command.ownerScopeGeneration,
      )
    ) {
      return false;
    }
    const mutationFailure = projectMutationFailure(error, "Goal 操作失败");
    const resourceFailure = getResourceFailure(error, "Goal 操作失败");
    const locked = mutationFailure.effect === "not_applied"
      ? null
      : {
          detail: mutationFailure.message,
          effect: mutationFailure.effect,
          intent: transaction.intent,
          ownerScopeGeneration: transaction.command.ownerScopeGeneration,
        } satisfies GoalMutationLock;
    updateMutationLock(locked);
    if (resourceFailure.access) {
      setSnapshot({
        binding: null,
        goal: null,
        loading: false,
        ownerScopeGeneration: transaction.command.ownerScopeGeneration,
        sessionKey: transaction.command.sessionKey,
      });
    }
    setReliability(reliabilityState({
      access: resourceFailure.access,
      blocksMutations: Boolean(resourceFailure.access),
      detail: mutationFailure.message,
      kind: mutationFailure.effect === "not_applied"
        ? "mutation_not_applied"
        : mutationFailure.effect === "accepted"
          ? "mutation_accepted"
          : mutationFailure.effect === "committed"
            ? "mutation_committed_refresh_failed"
            : "mutation_unknown",
      operation: transaction.intent.operation,
      ownerScopeGeneration: transaction.command.ownerScopeGeneration,
      sessionKey: transaction.command.sessionKey,
      stale: mutationFailure.effect !== "not_applied",
    }));
    return locked !== null;
  }, [updateMutationLock]);

  const finishCommand = useCallback((
    transaction: GoalCommandTransaction,
  ): boolean => {
    const finishedCommand = transaction.command;
    if (activeCommandRef.current?.id === finishedCommand.id) {
      activeCommandRef.current = null;
    }
    setCommand((current) => current?.id === finishedCommand.id ? null : current);
    const deferred = deferredRefreshRef.current?.sessionKey
        === finishedCommand.sessionKey
      && deferredRefreshRef.current.ownerScopeGeneration
        === finishedCommand.ownerScopeGeneration;
    if (deferred) {
      deferredRefreshRef.current = null;
    }
    return deferred;
  }, []);

  const runCommand = useCallback(async (
    input: GoalLifecycleMutationInput,
    action: (goalId: string) => Promise<Goal | null>,
  ): Promise<GoalCommandOutcome> => {
    const transaction = beginCommand(input);
    if (!transaction) {
      return { goal: null, ok: false };
    }

    let outcome: GoalCommandOutcome = { goal: null, ok: false };
    let reconcileAfterFailure = false;
    let synchronizeAfterSuccess = false;
    try {
      const updated = await action(transaction.goalId);
      if (!validGoalCommandResult(transaction, updated)) {
        throw new Error("Goal 操作已返回，但无法确认它属于当前会话和目标");
      }
      if (resolveCommand(transaction, updated)) {
        outcome = { goal: updated, ok: true };
        synchronizeAfterSuccess = true;
      }
    } catch (error) {
      reconcileAfterFailure = rejectCommand(transaction, error);
    }
    const deferred = finishCommand(transaction);
    if (synchronizeAfterSuccess) {
      void refreshResource({ committedOperation: transaction.intent.operation });
    } else if (reconcileAfterFailure) {
      void reconcileLockedMutation();
    } else if (deferred) {
      void refreshResource();
    }
    return outcome;
  }, [
    beginCommand,
    finishCommand,
    reconcileLockedMutation,
    refreshResource,
    rejectCommand,
    resolveCommand,
  ]);

  useEffect(() => {
    if (
      previousSessionKeyRef.current === sessionKey
      && previousOwnerScopeGenerationRef.current === ownerScopeGeneration
    ) {
      return;
    }
    previousSessionKeyRef.current = sessionKey;
    previousOwnerScopeGenerationRef.current = ownerScopeGeneration;
    requestVersionRef.current += 1;
    activeCommandRef.current = null;
    deferredRefreshRef.current = null;
    mutationLockRef.current = null;
    reconcilingRef.current = false;
    setCommand(null);
    setMutationLock(null);
    setReconciling(false);
    setReliability(null);
    setSnapshot(emptySnapshot(sessionKey, ownerScopeGeneration));
  }, [ownerScopeGeneration, sessionKey]);

  const mutationBlockReason: GoalMutationBlockReason | null = currentMutationLock
    ? "unknown_mutation"
    : currentReliability?.blocksMutations ? "stale_read" : null;

  return {
    executionBinding,
    goal,
    isLoading: visibleSnapshot.loading || currentCommand !== null || reconciling,
    mutationBlockReason,
    mutationsBlocked: currentMutationLock !== null
      || Boolean(currentReliability?.blocksMutations),
    ownerScopeGeneration,
    phase: currentCommand?.phase ?? null,
    refresh,
    reliability: currentReliability,
    runCommand,
  };
}
