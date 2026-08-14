import { useCallback, useRef } from "react";

import { getMessageSendAckTimeoutMs } from "@/config/conversation-policy";

type PendingRequestAck = {
  reject: (error: Error) => void;
  resolve: () => void;
  timeout_id: ReturnType<typeof globalThis.setTimeout>;
};

export interface RequestAcceptanceCorrelation {
  clientMessageId: string;
  clientRequestId: string;
  sessionKey: string;
}

export interface PendingRequestAckRegistry {
  pending: Map<string, PendingRequestAck>;
  preserved: Set<string>;
  rejected: Map<string, Error>;
  settled: Set<string>;
  tracked: Set<string>;
}

export class RequestAcceptanceUnknownError extends Error {
  readonly correlation: RequestAcceptanceCorrelation | null;

  constructor(
    message: string,
    correlation: RequestAcceptanceCorrelation | null = null,
  ) {
    super(message);
    this.name = "RequestAcceptanceUnknownError";
    this.correlation = correlation;
  }
}

export function createPendingRequestAckRegistry(): PendingRequestAckRegistry {
  return {
    pending: new Map(),
    preserved: new Set(),
    rejected: new Map(),
    settled: new Set(),
    tracked: new Set(),
  };
}

export function trackPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
  preserveAcrossSessionTransition = false,
): boolean {
  const normalizedRequestId = clientRequestId.trim();
  if (!normalizedRequestId) {
    return false;
  }
  registry.tracked.add(normalizedRequestId);
  if (preserveAcrossSessionTransition) {
    registry.preserved.add(normalizedRequestId);
  }
  return true;
}

function ownsPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
): boolean {
  return registry.tracked.has(clientRequestId)
    || registry.pending.has(clientRequestId)
    || registry.rejected.has(clientRequestId)
    || registry.settled.has(clientRequestId);
}

export function resolvePendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId?: string | null,
): boolean {
  if (!clientRequestId) {
    return false;
  }
  if (!ownsPendingRequestAck(registry, clientRequestId)) {
    return false;
  }
  registry.rejected.delete(clientRequestId);
  registry.preserved.delete(clientRequestId);
  const pendingRequest = registry.pending.get(clientRequestId);
  if (!pendingRequest) {
    registry.tracked.delete(clientRequestId);
    registry.settled.add(clientRequestId);
    return false;
  }
  globalThis.clearTimeout(pendingRequest.timeout_id);
  registry.pending.delete(clientRequestId);
  registry.settled.delete(clientRequestId);
  registry.tracked.delete(clientRequestId);
  pendingRequest.resolve();
  return true;
}

export function rejectPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
  cause: string | Error,
): boolean {
  if (!ownsPendingRequestAck(registry, clientRequestId)) {
    return false;
  }
  const error = cause instanceof Error ? cause : new Error(cause);
  registry.preserved.delete(clientRequestId);
  registry.settled.delete(clientRequestId);
  const pendingRequest = registry.pending.get(clientRequestId);
  if (!pendingRequest) {
    registry.tracked.delete(clientRequestId);
    registry.rejected.set(clientRequestId, error);
    return false;
  }
  globalThis.clearTimeout(pendingRequest.timeout_id);
  registry.pending.delete(clientRequestId);
  registry.tracked.delete(clientRequestId);
  pendingRequest.reject(error);
  return true;
}

export function hasPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
): boolean {
  return registry.pending.has(clientRequestId);
}

/** 发送尚未建立 waiter 就失败时，精确丢弃该 request owner 的全部痕迹。 */
export function discardPendingRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
): void {
  const pendingRequest = registry.pending.get(clientRequestId);
  if (pendingRequest) {
    globalThis.clearTimeout(pendingRequest.timeout_id);
    registry.pending.delete(clientRequestId);
  }
  registry.preserved.delete(clientRequestId);
  registry.rejected.delete(clientRequestId);
  registry.settled.delete(clientRequestId);
  registry.tracked.delete(clientRequestId);
}

export function cancelPendingRequestAcks(
  registry: PendingRequestAckRegistry,
  reason: string,
  keepPreserved = false,
): void {
  for (const [
    clientRequestId,
    pendingRequest,
  ] of registry.pending.entries()) {
    if (keepPreserved && registry.preserved.has(clientRequestId)) {
      continue;
    }
    globalThis.clearTimeout(pendingRequest.timeout_id);
    pendingRequest.reject(new RequestAcceptanceUnknownError(reason));
    registry.pending.delete(clientRequestId);
    registry.preserved.delete(clientRequestId);
  }
  for (const requestId of registry.settled) {
    if (!keepPreserved || !registry.preserved.has(requestId)) {
      registry.settled.delete(requestId);
      registry.preserved.delete(requestId);
    }
  }
  for (const requestId of registry.rejected.keys()) {
    if (!keepPreserved || !registry.preserved.has(requestId)) {
      registry.rejected.delete(requestId);
      registry.preserved.delete(requestId);
    }
  }
  for (const requestId of registry.tracked) {
    if (!keepPreserved || !registry.preserved.has(requestId)) {
      registry.tracked.delete(requestId);
      registry.preserved.delete(requestId);
    }
  }
  if (!keepPreserved) {
    registry.preserved.clear();
  }
}

export function waitForRequestAck(
  registry: PendingRequestAckRegistry,
  clientRequestId: string,
  onTimeout: () => void,
  timeoutMs = getMessageSendAckTimeoutMs(),
): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    trackPendingRequestAck(registry, clientRequestId);
    if (registry.settled.delete(clientRequestId)) {
      registry.tracked.delete(clientRequestId);
      registry.preserved.delete(clientRequestId);
      resolve();
      return;
    }
    const rejectedError = registry.rejected.get(clientRequestId);
    if (rejectedError) {
      registry.rejected.delete(clientRequestId);
      registry.tracked.delete(clientRequestId);
      registry.preserved.delete(clientRequestId);
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

  const trackRequestAck = useCallback((
    clientRequestId: string,
    preserveAcrossSessionTransition = false,
  ) => (
    trackPendingRequestAck(
      registryRef.current,
      clientRequestId,
      preserveAcrossSessionTransition,
    )
  ), []);

  const rejectRequestAck = useCallback((
    clientRequestId: string,
    reason: string | Error,
  ) => (
    rejectPendingRequestAck(registryRef.current, clientRequestId, reason)
  ), []);

  const discardRequestAck = useCallback((clientRequestId: string) => {
    discardPendingRequestAck(registryRef.current, clientRequestId);
  }, []);

  const hasRequestAck = useCallback((clientRequestId: string) => (
    hasPendingRequestAck(registryRef.current, clientRequestId)
  ), []);

  const cancelRequestAcks = useCallback((
    reason: string,
    keepPreserved = false,
  ) => {
    cancelPendingRequestAcks(registryRef.current, reason, keepPreserved);
  }, []);

  const waitForAck = useCallback((
    clientRequestId: string,
    onTimeout: () => void,
    timeoutMs?: number,
  ) => (
    waitForRequestAck(
      registryRef.current,
      clientRequestId,
      onTimeout,
      timeoutMs,
    )
  ), []);

  return {
    cancel_pending_request_acks: cancelRequestAcks,
    discard_pending_request_ack: discardRequestAck,
    has_pending_request_ack: hasRequestAck,
    reject_pending_request_ack: rejectRequestAck,
    resolve_pending_request_ack: resolveRequestAck,
    track_pending_request_ack: trackRequestAck,
    wait_for_request_ack: waitForAck,
  };
}
