/**
 * INPUT: exact Session/Goal mutation intent and a later owner-scoped current Goal read.
 * OUTPUT: conservative reconciliation evidence that never treats an unchanged snapshot as proof of rejection.
 * POS: Goal lifecycle mutation recovery model; it owns no transport, retry, global identity or durable journal.
 */

import type { Goal } from "@/types/conversation/goal";
import type { ResourceAccessFailure } from "@/lib/error-message";

export type GoalLifecycleOperation = "clear" | "pause" | "resume" | "update";
export type GoalMutationBlockReason = "stale_read" | "unknown_mutation";

export type GoalLifecycleMutationInput =
  | { operation: "clear" }
  | { operation: "pause" }
  | { operation: "resume" }
  | {
      objective: string;
      operation: "update";
      tokenBudget: number | null | undefined;
    };

interface GoalLifecycleIntentBase {
  baseContinuationState: Goal["continuation_state"];
  baseStatus: Goal["status"];
  baseVersion: number;
  operation: GoalLifecycleOperation;
  sessionKey: string;
  targetGoalId: string;
}

export type GoalLifecycleIntent =
  | (GoalLifecycleIntentBase & { operation: "clear" })
  | (GoalLifecycleIntentBase & { operation: "pause" })
  | (GoalLifecycleIntentBase & { operation: "resume" })
  | (GoalLifecycleIntentBase & {
      expectedObjective: string;
      expectedTokenBudget: number | null | undefined;
      operation: "update";
    });

export type GoalLifecycleReconcileOutcome =
  | "applied"
  | "target_not_current"
  | "unproven";

export type GoalReliabilityKind =
  | "access_lost"
  | "binding_failed"
  | "mutation_accepted"
  | "mutation_applied"
  | "mutation_committed"
  | "mutation_committed_refresh_failed"
  | "mutation_not_applied"
  | "mutation_reconcile_failed"
  | "mutation_target_not_current"
  | "mutation_unknown"
  | "mutation_unproven"
  | "read_failed"
  | "runtime_budget_limited"
  | "runtime_failed"
  | "runtime_usage_limited";

export interface GoalReliabilityState {
  access: ResourceAccessFailure | null;
  detail: string;
  kind: GoalReliabilityKind;
  operation: GoalLifecycleOperation | null;
  sessionKey: string;
  stale: boolean;
}

export function createGoalLifecycleIntent(
  goal: Goal,
  sessionKey: string,
  input: GoalLifecycleMutationInput,
): GoalLifecycleIntent {
  const base = {
    baseContinuationState: goal.continuation_state,
    baseStatus: goal.status,
    baseVersion: goal.version,
    sessionKey,
    targetGoalId: goal.id,
  };
  if (input.operation !== "update") {
    return { ...base, operation: input.operation };
  }
  return {
    ...base,
    expectedObjective: input.objective.trim(),
    expectedTokenBudget: input.tokenBudget,
    operation: "update",
  };
}

/**
 * A read can prove that the requested end state now exists, or that the exact
 * target is no longer this Session's current Goal. The latter only isolates a
 * new current Goal; the old request may still affect the old Goal's history. An
 * unchanged read cannot prove rejection because the request may commit later,
 * and Goal version may also advance for unrelated runtime facts.
 */
export function reconcileGoalLifecycleIntent(
  intent: GoalLifecycleIntent,
  current: Goal | null,
): GoalLifecycleReconcileOutcome {
  if (current && current.session_key !== intent.sessionKey) {
    return "unproven";
  }
  if (!current || current.id !== intent.targetGoalId) {
    return "target_not_current";
  }
  switch (intent.operation) {
    case "clear":
      return "unproven";
    case "pause":
      return current.status === "paused" ? "applied" : "unproven";
    case "resume":
      return resumeStateReached(intent, current) ? "applied" : "unproven";
    case "update":
      return updateStateReached(intent, current) ? "applied" : "unproven";
  }
}

function resumeStateReached(
  intent: Extract<GoalLifecycleIntent, { operation: "resume" }>,
  current: Goal,
): boolean {
  if (current.status !== "active") {
    return false;
  }
  return intent.baseStatus !== "active"
    || intent.baseContinuationState !== "suspended"
    || current.continuation_state !== "suspended";
}

function updateStateReached(
  intent: Extract<GoalLifecycleIntent, { operation: "update" }>,
  current: Goal,
): boolean {
  if (current.objective.trim() !== intent.expectedObjective) {
    return false;
  }
  if (intent.expectedTokenBudget === undefined) {
    return true;
  }
  return normalizeBudget(current.token_budget) === intent.expectedTokenBudget;
}

function normalizeBudget(value: number | null | undefined): number | null {
  return typeof value === "number" ? value : null;
}
