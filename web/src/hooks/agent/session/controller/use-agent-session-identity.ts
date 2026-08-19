import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";

import { getAgentConversationIdentityKey } from "@/lib/conversation/agent-conversation-identity";
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";

interface UseAgentSessionIdentityOptions {
  activeSessionKeyRef: RefObject<string | null>;
  cancelPendingRequestAcks: (
    reason: string,
    keepPreserved?: boolean,
  ) => void;
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
    cancelPendingRequestAcks(
      "会话已切换，未确认的页面请求已取消",
      true,
    );
    activeIdentityKeyRef.current = nextIdentityKey;
    sessionSeqCursorRef.current = 0;
    roomSeqCursorRef.current = 0;
    resetHistoryPagination();
    clearLiveSessionState();
    // 用户消息、排队输入和 Goal 由 durable client_request_id owner 继续
    // 收口；其余页面请求已取消，迟到结果不得投影进新会话。
    resetRuntimeMachine();
  }, [
    clearLiveSessionState,
    cancelPendingRequestAcks,
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
    cancelPendingRequestAcks(
      "会话已卸载，未确认的页面请求已取消",
      true,
    );
  }, [cancelPendingRequestAcks]);

  return { isCurrentSessionEvent };
}
