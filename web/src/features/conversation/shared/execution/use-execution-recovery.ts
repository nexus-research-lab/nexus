/**
 * INPUT: Current session/Execution and server-authorized recovery node identities.
 * OUTPUT: Scoped recovery actions with shared duplicate-click guards and read-only unknown-outcome reconciliation.
 * POS: DM/Room WorkGraph recovery state; queued acknowledgement never marks an Attempt running or completed.
 * A queued fence is released after a cooldown so a still-recoverable Attempt can be retried with a new intent.
 */
import { ApiRequestError } from "@/lib/api/core/http-error";
import { useEffect, useRef } from "react";

import { getExecutionRecoveryReceiptApi, requestExecutionRecoveryApi, getWorkRetryPreviewApi, requestWorkRetryApi } from "@/lib/api/conversation/execution-api";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { ExecutionGraphNodeView, WorkRetryPreview } from "@/types/conversation/execution";

/** Long enough to cover a typical coordinator recovery round; the server drops stale queued targets. */
export const RECOVERY_REQUEUE_MS = 120_000;

type RecoveryStatus = "confirm" | "pending" | "queued" | "unknown" | "absent" | "rejected";
interface RecoveryRequest { requestId: string; status: RecoveryStatus }
export interface ExecutionRecoveryAction {
  confirmation?: WorkRetryPreview | null;
  confirmRetry?: () => void;
  cancelRetry?: () => void;
  run: (node: ExecutionGraphNodeView) => Promise<void>;
  states: Readonly<Record<string, RecoveryRequest>>;
}

export function useExecutionRecovery(sessionKey: string, executionId: string, onRefresh: () => void): ExecutionRecoveryAction {
  const scope = `${sessionKey}\u0000${executionId}`;
  const [states, setStates] = useResettableState<Record<string, RecoveryRequest>>({}, scope);
  const [confirmation, setConfirmation] = useResettableState<WorkRetryPreview | null>(null, scope);
  const confirmed = useRef<{ preview: WorkRetryPreview; active: object } | null>(null);
  const current = useRef({ scope, requests: {} as Record<string, RecoveryRequest> });
  if (current.current.scope !== scope) current.current = { scope, requests: {} };
  useEffect(() => {
    if (current.current.scope !== scope) current.current = { scope, requests: {} };
    const active = current.current;
    return () => { if (current.current === active) current.current = { scope: "", requests: {} }; };
  }, [scope]);

  const run = async (node: ExecutionGraphNodeView, approved?: WorkRetryPreview) => {
    if ((!node.can_recover && !node.can_retry) || !node.work_item_id || !executionId || current.current.scope !== scope) return;
    const attemptId = node.attempt_id ?? "";
    const stateKey = attemptId || node.work_item_id;
    const active = current.current;
    const previous = active.requests[stateKey];
    if (previous?.status === "pending" || previous?.status === "queued" || (previous?.status === "confirm" && !approved)) return;
    const requestId = previous?.requestId ?? crypto.randomUUID();
    const update = (status: RecoveryStatus) => {
      if (current.current !== active) return;
      active.requests[stateKey] = { requestId, status };
      setStates({ ...active.requests });
    };
    update("pending");
    try {
      let preview = approved;
      if (node.can_retry && previous?.status !== "unknown" && !preview) {
        try {
          preview = await getWorkRetryPreviewApi(sessionKey, executionId, node.work_item_id, attemptId);
        } catch {
          update("rejected");
          if (current.current === active) onRefresh();
          return;
        }
        if (current.current !== active) return;
        if (preview.execution_id !== executionId || preview.work_item_id !== node.work_item_id || preview.attempt_id !== attemptId) {
          update("rejected");
          return;
        }
        if (preview.submitted) {
          update("confirm");
          confirmed.current = { preview, active };
          setConfirmation(preview);
          return;
        }
      }
      const receipt = previous?.status === "unknown"
        ? await getExecutionRecoveryReceiptApi(sessionKey, executionId, requestId)
        : preview ? await requestWorkRetryApi(sessionKey, preview, requestId)
        : await requestExecutionRecoveryApi(sessionKey, executionId, node.work_item_id, attemptId, requestId);
      if (receipt.request_id !== requestId || receipt.execution_id !== executionId || (receipt.queued && (receipt.work_item_id !== node.work_item_id || receipt.attempt_id !== attemptId))) {
        update("unknown");
        return;
      }
      update(receipt.queued ? "queued" : "absent");
      if (receipt.queued) {
        setTimeout(() => {
          if (current.current !== active || active.requests[stateKey]?.requestId !== requestId) return;
          delete active.requests[stateKey];
          setStates({ ...active.requests });
        }, RECOVERY_REQUEUE_MS);
      }
      if (current.current === active) onRefresh();
    } catch (error) {
      if (error instanceof ApiRequestError && error.status === 409) {
        update("rejected");
        if (current.current === active) onRefresh();
      } else update("unknown");
    }
  };
  const cancelRetry = () => {
    const pending = confirmed.current;
    if (pending?.active === current.current) {
      delete current.current.requests[pending.preview.attempt_id || pending.preview.work_item_id];
      setStates({ ...current.current.requests });
    }
    confirmed.current = null;
    setConfirmation(null);
  };
  const confirmRetry = () => {
    const pending = confirmed.current;
    if (!pending || pending.active !== current.current) return;
    confirmed.current = null;
    setConfirmation(null);
    void run({ id: pending.preview.work_item_id, can_retry: true, work_item_id: pending.preview.work_item_id, attempt_id: pending.preview.attempt_id } as ExecutionGraphNodeView, pending.preview);
  };
  return { states, run, confirmation, confirmRetry, cancelRetry };
}
