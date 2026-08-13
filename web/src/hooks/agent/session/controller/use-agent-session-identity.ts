import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";

import { getAgentConversationIdentityKey } from "@/lib/conversation/agent-conversation-identity";
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";

interface UseAgentSessionIdentityOptions {
  activeSessionKeyRef: RefObject<string | null>;
  cancelPendingRequestAcks: (reason: string) => void;
  clearLiveSessionState: () => void;
  identity: AgentConversationIdentity | null;
  identitySessionKey: string | null;
  resetHistoryPagination: () => void;
  resetRuntimeMachine: () => void;
  roomSeqCursorRef: RefObject<number>;
  sessionSeqCursorRef: RefObject<number>;
}

export function useAgentSessionIdentity({
  activeSessionKeyRef,
  cancelPendingRequestAcks,
  clearLiveSessionState,
  identity,
  identitySessionKey,
  resetHistoryPagination,
  resetRuntimeMachine,
  roomSeqCursorRef,
  sessionSeqCursorRef,
}: UseAgentSessionIdentityOptions): {
  isCurrentSessionEvent: (incomingSessionKey?: string | null) => boolean;
} {
  const activeIdentityKeyRef = useRef<string | null>(
    getAgentConversationIdentityKey(identity),
  );
  const isCurrentSessionEvent = useCallback(
    (incomingSessionKey?: string | null): boolean => (
      Boolean(incomingSessionKey) && areEquivalentSessionKeys(
        activeSessionKeyRef.current,
        incomingSessionKey,
      )
    ),
    [activeSessionKeyRef],
  );

  useEffect(() => {
    const nextIdentityKey = getAgentConversationIdentityKey(identity);
    if (activeIdentityKeyRef.current === nextIdentityKey) {
      return;
    }
    activeIdentityKeyRef.current = nextIdentityKey;
    sessionSeqCursorRef.current = 0;
    roomSeqCursorRef.current = 0;
    resetHistoryPagination();
    clearLiveSessionState();
    // 已发出的请求继续由 client_request_id 收口。切换会话只清理当前
    // 页面投影，不能把尚未到达的旧会话 ACK 伪装成发送失败。
    resetRuntimeMachine();
  }, [
    clearLiveSessionState,
    identity,
    resetHistoryPagination,
    roomSeqCursorRef,
    resetRuntimeMachine,
    sessionSeqCursorRef,
  ]);

  useEffect(() => {
    activeSessionKeyRef.current = identitySessionKey;
  }, [activeSessionKeyRef, identitySessionKey]);

  useEffect(() => () => {
    cancelPendingRequestAcks("会话已卸载，未确认的消息发送已取消");
  }, [cancelPendingRequestAcks]);

  return { isCurrentSessionEvent };
}
