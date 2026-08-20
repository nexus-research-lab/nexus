/**
 * INPUT: 会话身份、投影节点数量与 live layout epoch。
 * OUTPUT: 会话内单调启用、且不会在并行流式输出中途切换 renderer 的虚拟化决定。
 * POS: DM/Room static 与 virtual Feed 的共享切换协议。
 */
import { useEffect, useState } from "react";

const VIRTUALIZATION_SETTLE_DELAY_MS = 260;

interface VirtualizationDecision {
  enabled: boolean;
  scopeKey: string;
}

interface UseConversationVirtualizationPolicyOptions {
  active: boolean;
  count: number;
  scopeKey: string | null | undefined;
  threshold: number;
}

export function resolveInitialConversationVirtualization(
  count: number,
  threshold: number,
): boolean {
  return count >= threshold;
}

export function useConversationVirtualizationPolicy({
  active,
  count,
  scopeKey,
  threshold,
}: UseConversationVirtualizationPolicyOptions): boolean {
  const normalizedScopeKey = scopeKey ?? "";
  const [decision, setDecision] = useState<VirtualizationDecision>(() => ({
    enabled: resolveInitialConversationVirtualization(count, threshold),
    scopeKey: normalizedScopeKey,
  }));
  const effectiveEnabled = decision.scopeKey === normalizedScopeKey
    ? decision.enabled
    : resolveInitialConversationVirtualization(count, threshold);

  useEffect(() => {
    if (decision.scopeKey !== normalizedScopeKey) {
      setDecision({
        enabled: resolveInitialConversationVirtualization(count, threshold),
        scopeKey: normalizedScopeKey,
      });
      return;
    }
    if (effectiveEnabled || active || count < threshold) {
      return;
    }
    const timer = window.setTimeout(() => {
      setDecision((current) => current.scopeKey === normalizedScopeKey
        ? { ...current, enabled: true }
        : current);
    }, VIRTUALIZATION_SETTLE_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [
    active,
    count,
    decision.scopeKey,
    effectiveEnabled,
    normalizedScopeKey,
    threshold,
  ]);

  return effectiveEnabled;
}
