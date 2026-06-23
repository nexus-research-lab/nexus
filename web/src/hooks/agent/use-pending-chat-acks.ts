import { useCallback, useRef } from "react";

import { get_message_send_ack_timeout_ms } from "@/config/options";

type PendingChatAck = {
  reject: (error: Error) => void;
  resolve: () => void;
  timeout_id: number;
};

export function usePendingChatAcks() {
  const pending_chat_ack_ref = useRef<Map<string, PendingChatAck>>(new Map());

  const clear_pending_chat_ack = useCallback((round_id?: string | null) => {
    if (!round_id) {
      return false;
    }
    const pending_request = pending_chat_ack_ref.current.get(round_id);
    if (!pending_request) {
      return false;
    }
    window.clearTimeout(pending_request.timeout_id);
    pending_chat_ack_ref.current.delete(round_id);
    pending_request.resolve();
    return true;
  }, []);

  const reject_pending_chat_ack = useCallback((round_id: string, reason: string) => {
    const pending_request = pending_chat_ack_ref.current.get(round_id);
    if (!pending_request) {
      return false;
    }
    window.clearTimeout(pending_request.timeout_id);
    pending_chat_ack_ref.current.delete(round_id);
    pending_request.reject(new Error(reason));
    return true;
  }, []);

  const cancel_pending_chat_acks = useCallback((reason: string) => {
    for (const [
      round_id,
      pending_request,
    ] of pending_chat_ack_ref.current.entries()) {
      window.clearTimeout(pending_request.timeout_id);
      pending_request.reject(new Error(reason));
      pending_chat_ack_ref.current.delete(round_id);
    }
  }, []);

  const wait_for_chat_ack = useCallback((round_id: string, on_timeout: () => void) =>
    new Promise<void>((resolve, reject) => {
      const timeout_id = window.setTimeout(on_timeout, get_message_send_ack_timeout_ms());
      pending_chat_ack_ref.current.set(round_id, {
        resolve,
        reject,
        timeout_id,
      });
    }), []);

  return {
    cancel_pending_chat_acks,
    clear_pending_chat_ack,
    reject_pending_chat_ack,
    wait_for_chat_ack,
  };
}
