/**
 * INPUT: A Channel mutation error plus the exact user operation being attempted.
 * OUTPUT: Machine-evidence classification and localized result/impact/recovery copy.
 * POS: Channel failure projection boundary; raw transport/provider text and IDs never leave it.
 */
import { projectMutationFailure } from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

export type ChannelMutationOperation =
  | "account_delete"
  | "channel_delete"
  | "channel_save"
  | "login_start"
  | "pairing_create"
  | "pairing_delete"
  | "pairing_update"
  | "verify_code";

export type ChannelMutationEvidence =
  | "accepted"
  | "committed"
  | "not_applied"
  | "unknown";

export interface ChannelOperationIssue {
  effect: ChannelMutationEvidence;
  impact: string;
  message: string;
  nextStep: string;
  operation: ChannelMutationOperation;
  title: string;
  tone: "error" | "warning";
}

const OPERATION_LABEL_KEYS = {
  account_delete: "capability.channel_operation_account_delete",
  channel_delete: "capability.channel_operation_channel_delete",
  channel_save: "capability.channel_operation_channel_save",
  login_start: "capability.channel_operation_login_start",
  pairing_create: "capability.channel_operation_pairing_create",
  pairing_delete: "capability.channel_operation_pairing_delete",
  pairing_update: "capability.channel_operation_pairing_update",
  verify_code: "capability.channel_operation_verify_code",
} as const;

const IMPACT_KEYS = {
  account_delete: {
    accepted: "capability.channel_account_delete_accepted_impact",
    committed: "capability.channel_account_delete_committed_impact",
    not_applied: "capability.channel_account_delete_not_applied_impact",
    unknown: "capability.channel_account_delete_unknown_impact",
  },
  channel_delete: {
    accepted: "capability.channel_delete_accepted_impact",
    committed: "capability.channel_delete_committed_impact",
    not_applied: "capability.channel_delete_not_applied_impact",
    unknown: "capability.channel_delete_unknown_impact",
  },
  channel_save: {
    accepted: "capability.channel_save_accepted_impact",
    committed: "capability.channel_save_committed_impact",
    not_applied: "capability.channel_save_not_applied_impact",
    unknown: "capability.channel_save_unknown_impact",
  },
  login_start: {
    accepted: "capability.channel_login_start_accepted_impact",
    committed: "capability.channel_login_start_committed_impact",
    not_applied: "capability.channel_login_start_not_applied_impact",
    unknown: "capability.channel_login_start_unknown_impact",
  },
  pairing_create: {
    accepted: "capability.channel_pairing_create_accepted_impact",
    committed: "capability.channel_pairing_create_committed_impact",
    not_applied: "capability.channel_pairing_create_not_applied_impact",
    unknown: "capability.channel_pairing_create_unknown_impact",
  },
  pairing_delete: {
    accepted: "capability.channel_pairing_delete_accepted_impact",
    committed: "capability.channel_pairing_delete_committed_impact",
    not_applied: "capability.channel_pairing_delete_not_applied_impact",
    unknown: "capability.channel_pairing_delete_unknown_impact",
  },
  pairing_update: {
    accepted: "capability.channel_pairing_update_accepted_impact",
    committed: "capability.channel_pairing_update_committed_impact",
    not_applied: "capability.channel_pairing_update_not_applied_impact",
    unknown: "capability.channel_pairing_update_unknown_impact",
  },
  verify_code: {
    accepted: "capability.channel_verify_code_accepted_impact",
    committed: "capability.channel_verify_code_committed_impact",
    not_applied: "capability.channel_verify_code_not_applied_impact",
    unknown: "capability.channel_verify_code_unknown_impact",
  },
} as const;

const EFFECT_COPY_KEYS = {
  accepted: {
    message: "capability.channel_operation_accepted_message",
    nextStep: "capability.channel_operation_accepted_next_step",
    title: "capability.channel_operation_accepted_title",
  },
  committed: {
    message: "capability.channel_operation_committed_message",
    nextStep: "capability.channel_operation_committed_next_step",
    title: "capability.channel_operation_committed_title",
  },
  not_applied: {
    message: "capability.channel_operation_not_applied_message",
    nextStep: "capability.channel_operation_not_applied_next_step",
    title: "capability.channel_operation_not_applied_title",
  },
  unknown: {
    message: "capability.channel_operation_unknown_message",
    nextStep: "capability.channel_operation_unknown_next_step",
    title: "capability.channel_operation_unknown_title",
  },
} as const;

export function buildChannelOperationIssue(
  error: unknown,
  operation: ChannelMutationOperation,
  t: I18nContextValue["t"],
): ChannelOperationIssue {
  // Only machine evidence is consumed. The fallback and server/provider detail
  // are intentionally discarded so secrets and implementation text cannot
  // become product copy.
  const failure = projectMutationFailure(
    error,
    t("capability.channel_operation_failed_fallback"),
  );
  const effect = failure.effect;
  const operationLabel = t(OPERATION_LABEL_KEYS[operation]);
  const copy = EFFECT_COPY_KEYS[effect];
  return {
    effect,
    impact: t(IMPACT_KEYS[operation][effect]),
    message: t(copy.message, { operation: operationLabel }),
    nextStep: t(copy.nextStep),
    operation,
    title: t(copy.title, { operation: operationLabel }),
    tone: effect === "not_applied" ? "error" : "warning",
  };
}

export function channelOperationNeedsReconciliation(
  issue: ChannelOperationIssue,
): boolean {
  return issue.effect !== "not_applied";
}
