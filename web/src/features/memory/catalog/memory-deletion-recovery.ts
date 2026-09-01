/**
 * INPUT: exact owner generation + Agent + Memory path 的删除结果证据与目录核对状态。
 * OUTPUT: 是否允许重试、核对或显式开始新删除意图的纯恢复投影。
 * POS: Memory Catalog 删除可靠性模型；普通目录刷新不得自行解除 outcome_unknown。
 */
import type { MutationFailure } from "@/lib/error-message";

export type MemoryDeletionDirectoryCheck =
  | "failed"
  | "not_checked"
  | "target_present";

export type MemoryDeletionIssueKind =
  | "committed_refresh"
  | "not_applied"
  | "outcome_unknown";

export interface MemoryDeletionIssue {
  agentId: string;
  directoryCheck: MemoryDeletionDirectoryCheck;
  kind: MemoryDeletionIssueKind;
  ownerGeneration: number;
  path: string;
  title: string;
}

export type MemoryDeletionRecoveryAction =
  | "reconcile"
  | "retry"
  | "start_new_intent";

export interface MemoryDeletionRecoveryPresentation {
  impactKey:
    | "capability.memory_delete_committed_checking_impact"
    | "capability.memory_delete_committed_refresh_failed_impact"
    | "capability.memory_delete_committed_target_present_impact"
    | "capability.memory_delete_not_applied_impact"
    | "capability.memory_delete_unknown_check_failed_impact"
    | "capability.memory_delete_unknown_impact"
    | "capability.memory_delete_unknown_target_present_impact";
  nextStepKey:
    | "capability.memory_delete_committed_checking_next_step"
    | "capability.memory_delete_committed_refresh_failed_next_step"
    | "capability.memory_delete_committed_target_present_next_step"
    | "capability.memory_delete_not_applied_next_step"
    | "capability.memory_delete_unknown_check_failed_next_step"
    | "capability.memory_delete_unknown_next_step"
    | "capability.memory_delete_unknown_target_present_next_step";
  primaryAction: MemoryDeletionRecoveryAction;
  titleKey:
    | "capability.memory_delete_committed_checking_title"
    | "capability.memory_delete_committed_refresh_failed_title"
    | "capability.memory_delete_committed_target_present_title"
    | "capability.memory_delete_not_applied_title"
    | "capability.memory_delete_unknown_check_failed_title"
    | "capability.memory_delete_unknown_title"
    | "capability.memory_delete_unknown_target_present_title";
  tone: "error" | "warning";
}

export function projectMemoryDeletionFailure(
  failure: Pick<MutationFailure, "effect">,
  identity: Pick<MemoryDeletionIssue, "agentId" | "ownerGeneration" | "path" | "title">,
): MemoryDeletionIssue {
  return {
    ...identity,
    directoryCheck: "not_checked",
    kind: failure.effect === "committed"
      ? "committed_refresh"
      : failure.effect === "not_applied"
        ? "not_applied"
        : "outcome_unknown",
  };
}

export function projectCommittedMemoryDeletion(
  identity: Pick<MemoryDeletionIssue, "agentId" | "ownerGeneration" | "path" | "title">,
): MemoryDeletionIssue {
  return {
    ...identity,
    directoryCheck: "not_checked",
    kind: "committed_refresh",
  };
}

export function getMemoryDeletionRecoveryPresentation(
  issue: MemoryDeletionIssue,
): MemoryDeletionRecoveryPresentation {
  if (issue.kind === "not_applied") {
    return {
      impactKey: "capability.memory_delete_not_applied_impact",
      nextStepKey: "capability.memory_delete_not_applied_next_step",
      primaryAction: "retry",
      titleKey: "capability.memory_delete_not_applied_title",
      tone: "error",
    };
  }
  if (issue.kind === "committed_refresh") {
    if (issue.directoryCheck === "target_present") {
      return {
        impactKey: "capability.memory_delete_committed_target_present_impact",
        nextStepKey: "capability.memory_delete_committed_target_present_next_step",
        primaryAction: "start_new_intent",
        titleKey: "capability.memory_delete_committed_target_present_title",
        tone: "warning",
      };
    }
    if (issue.directoryCheck === "not_checked") {
      return {
        impactKey: "capability.memory_delete_committed_checking_impact",
        nextStepKey: "capability.memory_delete_committed_checking_next_step",
        primaryAction: "reconcile",
        titleKey: "capability.memory_delete_committed_checking_title",
        tone: "warning",
      };
    }
    return {
      impactKey: "capability.memory_delete_committed_refresh_failed_impact",
      nextStepKey: "capability.memory_delete_committed_refresh_failed_next_step",
      primaryAction: "reconcile",
      titleKey: "capability.memory_delete_committed_refresh_failed_title",
      tone: "warning",
    };
  }
  if (issue.directoryCheck === "target_present") {
    return {
      impactKey: "capability.memory_delete_unknown_target_present_impact",
      nextStepKey: "capability.memory_delete_unknown_target_present_next_step",
      primaryAction: "start_new_intent",
      titleKey: "capability.memory_delete_unknown_target_present_title",
      tone: "warning",
    };
  }
  if (issue.directoryCheck === "failed") {
    return {
      impactKey: "capability.memory_delete_unknown_check_failed_impact",
      nextStepKey: "capability.memory_delete_unknown_check_failed_next_step",
      primaryAction: "reconcile",
      titleKey: "capability.memory_delete_unknown_check_failed_title",
      tone: "warning",
    };
  }
  return {
    impactKey: "capability.memory_delete_unknown_impact",
    nextStepKey: "capability.memory_delete_unknown_next_step",
    primaryAction: "reconcile",
    titleKey: "capability.memory_delete_unknown_title",
    tone: "warning",
  };
}

export function canStartNewMemoryDeletionIntent(
  issue: MemoryDeletionIssue,
): boolean {
  return issue.directoryCheck === "target_present";
}

export function upsertMemoryDeletionIssue(
  issues: MemoryDeletionIssue[],
  issue: MemoryDeletionIssue,
): MemoryDeletionIssue[] {
  return [
    ...issues.filter((candidate) => candidate.path !== issue.path),
    issue,
  ];
}

export function removeMemoryDeletionIssue(
  issues: MemoryDeletionIssue[],
  path: string,
): MemoryDeletionIssue[] {
  return issues.filter((issue) => issue.path !== path);
}
