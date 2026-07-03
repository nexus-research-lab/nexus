import { useCallback, useRef } from "react";

import { getMessageSendAckTimeoutMs } from "@/config/options";

type PendingChatAck = {
  reject: (error: Error) => void;
  resolve: () => void;
  timeout_id: number;
};

export function usePendingChatAcks() {
  const pendingChatAckRef = useRef<Map<string, PendingChatAck>>(new Map());

  const clearPendingChatAck = useCallback((roundId?: string | null) => {
    if (!roundId) {
      return false;
    }
    const pendingRequest = pendingChatAckRef.current.get(roundId);
    if (!pendingRequest) {
      return false;
    }
    window.clearTimeout(pendingRequest.timeout_id);
    pendingChatAckRef.current.delete(roundId);
    pendingRequest.resolve();
    return true;
  }, []);

  const rejectPendingChatAck = useCallback((roundId: string, reason: string) => {
    const pendingRequest = pendingChatAckRef.current.get(roundId);
    if (!pendingRequest) {
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
  }, []);

  const waitForChatAck = useCallback((roundId: string, onTimeout: () => void) =>
    new Promise<void>((resolve, reject) => {
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
