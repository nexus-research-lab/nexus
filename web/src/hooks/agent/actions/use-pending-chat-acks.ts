import { useCallback, useRef } from "react";

import { getMessageSendAckTimeoutMs } from "@/config/conversation-policy";

type PendingChatAck = {
  reject: (error: Error) => void;
  resolve: () => void;
  timeout_id: number;
};

export function usePendingChatAcks() {
  const pendingChatAckRef = useRef<Map<string, PendingChatAck>>(new Map());
  const settledChatAckRef = useRef<Set<string>>(new Set());
  const rejectedChatAckRef = useRef<Map<string, string>>(new Map());

  const clearPendingChatAck = useCallback((roundId?: string | null) => {
    if (!roundId) {
      return false;
    }
    rejectedChatAckRef.current.delete(roundId);
    const pendingRequest = pendingChatAckRef.current.get(roundId);
    if (!pendingRequest) {
      settledChatAckRef.current.add(roundId);
      return false;
    }
    window.clearTimeout(pendingRequest.timeout_id);
    pendingChatAckRef.current.delete(roundId);
    pendingRequest.resolve();
    return true;
  }, []);

  const rejectPendingChatAck = useCallback((roundId: string, reason: string) => {
    settledChatAckRef.current.delete(roundId);
    const pendingRequest = pendingChatAckRef.current.get(roundId);
    if (!pendingRequest) {
      rejectedChatAckRef.current.set(roundId, reason);
      return false;
    }
    window.clearTimeout(pendingRequest.timeout_id);
    pendingChatAckRef.current.delete(roundId);
    pendingRequest.reject(new Error(reason));
    return true;
  }, []);

  const cancelPendingChatAcks = useCallback((reason: string) => {
    for (const [
      roundId,
      pendingRequest,
    ] of pendingChatAckRef.current.entries()) {
      window.clearTimeout(pendingRequest.timeout_id);
      pendingRequest.reject(new Error(reason));
      pendingChatAckRef.current.delete(roundId);
    }
    settledChatAckRef.current.clear();
    rejectedChatAckRef.current.clear();
  }, []);

  const waitForChatAck = useCallback((roundId: string, onTimeout: () => void) =>
    new Promise<void>((resolve, reject) => {
      if (settledChatAckRef.current.delete(roundId)) {
        resolve();
        return;
      }
      const rejectedReason = rejectedChatAckRef.current.get(roundId);
      if (rejectedReason) {
        rejectedChatAckRef.current.delete(roundId);
        reject(new Error(rejectedReason));
        return;
      }
      const timeoutId = window.setTimeout(onTimeout, getMessageSendAckTimeoutMs());
      pendingChatAckRef.current.set(roundId, {
        resolve,
        reject,
        timeout_id: timeoutId,
      });
    }), []);

  return {
    cancel_pending_chat_acks: cancelPendingChatAcks,
    clear_pending_chat_ack: clearPendingChatAck,
    reject_pending_chat_ack: rejectPendingChatAck,
    wait_for_chat_ack: waitForChatAck,
  };
}
