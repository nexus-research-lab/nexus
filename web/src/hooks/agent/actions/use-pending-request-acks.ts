import { useCallback, useRef } from "react";

import { getMessageSendAckTimeoutMs } from "@/config/conversation-policy";

type PendingRequestAck = {
  reject: (error: Error) => void;
  resolve: () => void;
  timeout_id: ReturnType<typeof globalThis.setTimeout>;
};

export interface PendingRequestAckRegistry {
  pending: Map<string, PendingRequestAck>;
  rejected: Map<string, Error>;
  settled: Set<string>;
}

export function createPendingRequestAckRegistry(): PendingRequestAckRegistry {
  return {
    pending: new Map(),
    rejected: new Map(),
    settled: new Set(),
  };
}

export function resolvePendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId?: string | null,
): boolean {
  if (!clientRequestId) {
    return false;
  }
  registry.rejected.delete(clientRequestId);
  const pendingRequest = registry.pending.get(clientRequestId);
  if (!pendingRequest) {
    registry.settled.add(clientRequestId);
    return false;
  }
  globalThis.clearTimeout(pendingRequest.timeout_id);
  registry.pending.delete(clientRequestId);
  pendingRequest.resolve();
  return true;
}

export function rejectPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
  cause: string | Error,
): boolean {
  const error = cause instanceof Error ? cause : new Error(cause);
  registry.settled.delete(clientRequestId);
  const pendingRequest = registry.pending.get(clientRequestId);
  if (!pendingRequest) {
    registry.rejected.set(clientRequestId, error);
    return false;
  }
  globalThis.clearTimeout(pendingRequest.timeout_id);
  registry.pending.delete(clientRequestId);
  pendingRequest.reject(error);
  return true;
}

export function hasPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
): boolean {
  return registry.pending.has(clientRequestId);
}

export function cancelPendingRequestAcks(
  registry: PendingRequestAckRegistry,
  reason: string,
): void {
  for (const [
    clientRequestId,
    pendingRequest,
  ] of registry.pending.entries()) {
    globalThis.clearTimeout(pendingRequest.timeout_id);
    pendingRequest.reject(new Error(reason));
    registry.pending.delete(clientRequestId);
  }
  registry.settled.clear();
  registry.rejected.clear();
}

export function waitForRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
  onTimeout: () => void,
  timeoutMs = getMessageSendAckTimeoutMs(),
): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    if (registry.settled.delete(clientRequestId)) {
      resolve();
      return;
    }
    const rejectedError = registry.rejected.get(clientRequestId);
    if (rejectedError) {
      registry.rejected.delete(clientRequestId);
      reject(rejectedError);
      return;
    }
    const timeoutId = globalThis.setTimeout(onTimeout, timeoutMs);
    registry.pending.set(clientRequestId, {
      resolve,
      reject,
      timeout_id: timeoutId,
    });
  });
}

export function usePendingRequestAcks() {
  const registryRef = useRef<PendingRequestAckRegistry>(
    createPendingRequestAckRegistry(),
  );

  const resolveRequestAck = useCallback((clientRequestId?: string | null) => (
    resolvePendingRequestAck(registryRef.current, clientRequestId)
  ), []);

  const rejectRequestAck = useCallback((
    clientRequestId: string,
    reason: string | Error,
  ) => (
    rejectPendingRequestAck(registryRef.current, clientRequestId, reason)
  ), []);

  const hasRequestAck = useCallback((clientRequestId: string) => (
    hasPendingRequestAck(registryRef.current, clientRequestId)
  ), []);

  const cancelRequestAcks = useCallback((reason: string) => {
    cancelPendingRequestAcks(registryRef.current, reason);
  }, []);

  const waitForAck = useCallback((
    clientRequestId: string,
    onTimeout: () => void,
  ) => (
    waitForRequestAck(registryRef.current, clientRequestId, onTimeout)
  ), []);

  return {
    cancel_pending_request_acks: cancelRequestAcks,
    has_pending_request_ack: hasRequestAck,
    reject_pending_request_ack: rejectRequestAck,
    resolve_pending_request_ack: resolveRequestAck,
    wait_for_request_ack: waitForAck,
  };
}
