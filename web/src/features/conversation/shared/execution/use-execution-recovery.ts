/**
 * INPUT: Current session/Execution and server-authorized recovery node identities.
 * OUTPUT: Scoped recovery actions with shared duplicate-click guards and read-only unknown-outcome reconciliation.
 * POS: DM/Room WorkGraph recovery state; queued acknowledgement never marks an Attempt running or completed.
 */
import { useEffect, useRef } from "react";

import { getExecutionRecoveryReceiptApi, requestExecutionRecoveryApi } from "@/lib/api/conversation/execution-api";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { ExecutionGraphNodeView } from "@/types/conversation/execution";

type RecoveryStatus = "pending" | "queued" | "unknown" | "absent";
interface RecoveryRequest { requestId: string; status: RecoveryStatus }
export interface ExecutionRecoveryAction {
  run: (node: ExecutionGraphNodeView) => void;
  states: Readonly<Record<string, RecoveryRequest>>;
}

export function useExecutionRecovery(sessionKey: string, executionId: string, onRefresh: () => void): ExecutionRecoveryAction {
  const scope = `${sessionKey}\u0000${executionId}`;
  const [states, setStates] = useResettableState<Record<string, RecoveryRequest>>({}, scope);
  const current = useRef({ scope, requests: {} as Record<string, RecoveryRequest> });
  if (current.current.scope !== scope) current.current = { scope, requests: {} };
  useEffect(() => {
    if (current.current.scope !== scope) current.current = { scope, requests: {} };
    const active = current.current;
    return () => { if (current.current === active) current.current = { scope: "", requests: {} }; };
  }, [scope]);

  const run = async (node: ExecutionGraphNodeView) => {
    if (!node.can_recover || !node.attempt_id || !node.work_item_id || !executionId || current.current.scope !== scope) return;
    const active = current.current;
    const previous = active.requests[node.attempt_id];
    if (previous?.status === "pending" || previous?.status === "queued") return;
    const requestId = previous?.requestId ?? crypto.randomUUID();
    const update = (status: RecoveryStatus) => {
      if (current.current !== active) return;
      active.requests[node.attempt_id!] = { requestId, status };
      setStates({ ...active.requests });
    };
    update("pending");
    try {
      const receipt = previous?.status === "unknown"
        ? await getExecutionRecoveryReceiptApi(sessionKey, executionId, requestId)
        : await requestExecutionRecoveryApi(sessionKey, executionId, node.work_item_id, node.attempt_id, requestId);
      if (receipt.request_id !== requestId || receipt.execution_id !== executionId || (receipt.queued && (receipt.work_item_id !== node.work_item_id || receipt.attempt_id !== node.attempt_id))) {
        update("unknown");
        return;
      }
      update(receipt.queued ? "queued" : "absent");
      if (current.current === active) onRefresh();
    } catch {
      update("unknown");
    }
  };
  return { states, run: (node) => { void run(node); } };
}
