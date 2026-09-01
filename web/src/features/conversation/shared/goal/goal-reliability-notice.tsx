/**
 * INPUT: one scope-fenced Goal reliability fact and the safe read-only reconciliation action.
 * OUTPUT: compact Problem / Impact / Recovery copy without exposing IDs or replaying mutations.
 * POS: Goal panel reliability renderer; mutation evidence remains owned by use-goal-resource.
 */
"use client";

import { RefreshCw } from "lucide-react";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type {
  GoalLifecycleOperation,
  GoalReliabilityState,
} from "./goal-lifecycle-recovery";

interface GoalReliabilityCopy {
  impact: string;
  nextStep: string;
  problem: string;
  tone: "error" | "info" | "warning";
}

const OPERATION_LABEL_KEYS: Record<GoalLifecycleOperation, TranslationKey> = {
  clear: "goal.reliability.operation_clear",
  pause: "goal.reliability.operation_pause",
  resume: "goal.reliability.operation_resume",
  update: "goal.reliability.operation_update",
};

const RECONCILABLE_KINDS = new Set<GoalReliabilityState["kind"]>([
  "access_lost",
  "binding_failed",
  "mutation_accepted",
  "mutation_committed_refresh_failed",
  "mutation_not_applied",
  "mutation_reconcile_failed",
  "mutation_target_not_current",
  "mutation_unknown",
  "mutation_unproven",
  "read_failed",
]);

export function GoalReliabilityNotice({
  className,
  isRefreshing,
  mutationBlocked,
  onRefresh,
  state,
}: {
  className?: string;
  isRefreshing: boolean;
  mutationBlocked: boolean;
  onRefresh: () => void;
  state: GoalReliabilityState;
}) {
  const { t } = useI18n();
  const copy = resolveGoalReliabilityCopy(t, state);
  const canRefresh = state.stale || RECONCILABLE_KINDS.has(state.kind);
  const commonProps = {
    "aria-label": copy.problem,
    className: cn("min-h-0 px-3 py-2", className),
    "data-goal-mutation-blocked": mutationBlocked ? "true" : undefined,
    "data-goal-reliability-kind": state.kind,
    impact: copy.impact,
    nextStep: copy.nextStep,
    size: "sm" as const,
    title: copy.problem,
    urgency: "polite" as const,
    variant: "card" as const,
  };
  if (copy.tone === "info") {
    return <UiResourceState {...commonProps} state="success" />;
  }
  return (
    <UiResourceState
      {...commonProps}
      description={state.detail}
      primaryAction={canRefresh ? {
        busy: isRefreshing,
        busyLabel: t("state.reload_check"),
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t("state.reload_check"),
        onClick: onRefresh,
      } : undefined}
      state="error"
      tone={copy.tone === "error" ? "danger" : "warning"}
    />
  );
}

function resolveGoalReliabilityCopy(
  t: I18nContextValue["t"],
  state: GoalReliabilityState,
): GoalReliabilityCopy {
  const operation = state.operation
    ? t(OPERATION_LABEL_KEYS[state.operation])
    : t("goal.reliability.operation_generic");
  if (state.kind === "mutation_not_applied") {
    return {
      impact: t("goal.reliability.not_applied_impact"),
      nextStep: state.access
        ? t(state.access === "authentication_required"
          ? "goal.reliability.authentication_next_step"
          : "goal.reliability.authorization_next_step")
        : t("goal.reliability.not_applied_next_step"),
      problem: t("goal.reliability.not_applied_problem", { operation }),
      tone: "warning",
    };
  }
  if (state.kind === "mutation_committed_refresh_failed") {
    return {
      impact: t("goal.reliability.committed_refresh_impact"),
      nextStep: state.access
        ? t(state.access === "authentication_required"
          ? "goal.reliability.authentication_next_step"
          : "goal.reliability.authorization_next_step")
        : t("goal.reliability.committed_refresh_next_step"),
      problem: t("goal.reliability.committed_refresh_problem", { operation }),
      tone: "warning",
    };
  }
  if (state.access) {
    return {
      impact: t(state.operation
        ? "goal.reliability.access_mutation_impact"
        : "goal.reliability.access_impact", { operation }),
      nextStep: t(state.access === "authentication_required"
        ? "goal.reliability.authentication_next_step"
        : "goal.reliability.authorization_next_step"),
      problem: t("goal.reliability.access_problem"),
      tone: "error",
    };
  }
  switch (state.kind) {
    case "read_failed":
      return {
        impact: t(state.stale
          ? "goal.reliability.read_stale_impact"
          : "goal.reliability.read_empty_impact"),
        nextStep: t("goal.reliability.read_next_step"),
        problem: t("goal.reliability.read_problem"),
        tone: "warning",
      };
    case "binding_failed":
      return {
        impact: t("goal.reliability.binding_impact"),
        nextStep: t("goal.reliability.binding_next_step"),
        problem: t("goal.reliability.binding_problem"),
        tone: "warning",
      };
    case "mutation_accepted":
      return {
        impact: t("goal.reliability.accepted_impact"),
        nextStep: t("goal.reliability.accepted_next_step"),
        problem: t("goal.reliability.accepted_problem", { operation }),
        tone: "warning",
      };
    case "mutation_unknown":
      return {
        impact: t("goal.reliability.unknown_impact"),
        nextStep: t("goal.reliability.unknown_next_step"),
        problem: t("goal.reliability.unknown_problem", { operation }),
        tone: "warning",
      };
    case "mutation_reconcile_failed":
      return {
        impact: t("goal.reliability.reconcile_failed_impact"),
        nextStep: t("goal.reliability.reconcile_failed_next_step"),
        problem: t("goal.reliability.reconcile_failed_problem", { operation }),
        tone: "warning",
      };
    case "mutation_unproven":
      return {
        impact: t("goal.reliability.unproven_impact"),
        nextStep: t("goal.reliability.unproven_next_step"),
        problem: t("goal.reliability.unproven_problem", { operation }),
        tone: "warning",
      };
    case "mutation_applied":
      return {
        impact: t(state.stale
          ? "goal.reliability.applied_partial_impact"
          : "goal.reliability.applied_impact"),
        nextStep: t(state.stale
          ? "goal.reliability.binding_next_step"
          : "goal.reliability.applied_next_step"),
        problem: t("goal.reliability.applied_problem", { operation }),
        tone: "info",
      };
    case "mutation_committed":
      return {
        impact: t("goal.reliability.committed_impact"),
        nextStep: t("goal.reliability.committed_next_step"),
        problem: t("goal.reliability.committed_problem", { operation }),
        tone: "info",
      };
    case "mutation_target_not_current":
      return {
        impact: t(state.stale
          ? "goal.reliability.target_not_current_partial_impact"
          : "goal.reliability.target_not_current_impact"),
        nextStep: t(state.stale
          ? "goal.reliability.binding_next_step"
          : "goal.reliability.target_not_current_next_step"),
        problem: t("goal.reliability.target_not_current_problem"),
        tone: "warning",
      };
    case "runtime_failed":
      return {
        impact: t("goal.reliability.runtime_impact"),
        nextStep: t("goal.reliability.runtime_next_step"),
        problem: t("goal.reliability.runtime_problem"),
        tone: "error",
      };
    case "runtime_budget_limited":
      return {
        impact: t("goal.reliability.runtime_budget_impact"),
        nextStep: t("goal.reliability.runtime_budget_next_step"),
        problem: t("goal.reliability.runtime_budget_problem"),
        tone: "warning",
      };
    case "runtime_usage_limited":
      return {
        impact: t("goal.reliability.runtime_usage_impact"),
        nextStep: t("goal.reliability.runtime_usage_next_step"),
        problem: t("goal.reliability.runtime_usage_problem"),
        tone: "warning",
      };
    case "access_lost":
      return {
        impact: t(state.operation
          ? "goal.reliability.access_mutation_impact"
          : "goal.reliability.access_impact", { operation }),
        nextStep: t("goal.reliability.authorization_next_step"),
        problem: t("goal.reliability.access_problem"),
        tone: "error",
      };
  }
}
