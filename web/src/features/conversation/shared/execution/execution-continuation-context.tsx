/**
 * INPUT: Current authoritative Execution resource and exact rendered Agent round identity.
 * OUTPUT: Shared continuation actions for the timeline and full graph, fenced by session and freshness.
 * POS: Conversation-scoped continuation owner; never matches responsibility by Agent name alone.
 */
import { ExecutionRetryConfirmation } from "./execution-retry-confirmation";
import { type ReactNode } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import type { ExecutionResource } from "./use-execution-resource";
import { useExecutionRecovery } from "./use-execution-recovery";

import { ContinuationContext, useSharedExecutionContinuation } from "./execution-continuation-state";

export function ExecutionContinuationProvider({ resource, children }: { resource: ExecutionResource; children: ReactNode }) {
  const action = useExecutionRecovery(resource.sessionKey ?? "", resource.execution?.id ?? "", resource.refresh);
  return <ContinuationContext.Provider value={{ resource, action }}>{children}<ExecutionRetryConfirmation action={action} /></ContinuationContext.Provider>;
}


export function ExecutionRoundContinueButton({ agentId, agentRoundId }: { agentId: string; agentRoundId: string }) {
  const context = useSharedExecutionContinuation();
  const { t } = useI18n();
  if (!context || context.resource.isStale || context.resource.isLoading || !agentRoundId) return null;
  const matches = context.resource.execution?.graph?.nodes?.filter((node) => (node.can_recover || node.can_retry) && node.agent_id === agentId && node.agent_round_id === agentRoundId) ?? [];
  if (matches.length !== 1) return null;
  const node = matches[0];
  const state = node.attempt_id ? context.action.states[node.attempt_id]?.status : undefined;
  const label = state === "unknown" ? "execution.recovery_check" : state === "pending" ? "execution.recovery_pending" : state === "queued" ? "execution.recovery_queued" : state === "rejected" ? "execution.recovery_rejected" : node.can_retry ? "execution.retry_node" : "execution.recover_node";
  return <UiButton size="xs" variant="text" disabled={state === "pending" || state === "queued"} onClick={() => context.action.run(node)}>{t(label)}</UiButton>;
}
