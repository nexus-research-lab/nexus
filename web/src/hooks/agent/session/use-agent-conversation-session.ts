import {
  useCallback,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import {
  clearAgentSession,
  loadAgentSession,
  resetAgentSession,
  startAgentSession,
  type AgentConversationLifecycleContext,
} from "./conversation-lifecycle";

interface UseAgentConversationSessionParams {
  activeSessionKeyRef: RefObject<string | null>;
  cancelPendingRequestAcks: (reason: string) => void;
  clearLiveSessionState: () => void;
  lifecycleContext: AgentConversationLifecycleContext;
  resetHistoryPagination: () => void;
  resetRuntimeMachine: () => void;
  setIsSessionLoading: Dispatch<SetStateAction<boolean>>;
  setSessionKey: Dispatch<SetStateAction<string | null>>;
}

type SessionTransition = (context: AgentConversationLifecycleContext) => void;

/** 管理会话键切换，并统一清理依附于旧会话的瞬时状态。 */
export function useAgentConversationSession({
  activeSessionKeyRef,
  cancelPendingRequestAcks,
  clearLiveSessionState,
  lifecycleContext,
  resetHistoryPagination,
  resetRuntimeMachine,
  setIsSessionLoading,
  setSessionKey,
}: UseAgentConversationSessionParams) {
  const runSessionTransition = useCallback(
    (reason: string, transition: SessionTransition): void => {
      cancelPendingRequestAcks(reason);
      clearLiveSessionState();
      transition(lifecycleContext);
      resetHistoryPagination();
      resetRuntimeMachine();
    },
    [
      cancelPendingRequestAcks,
      clearLiveSessionState,
      lifecycleContext,
      resetHistoryPagination,
      resetRuntimeMachine,
    ],
  );

  const startSession = useCallback((): void => {
    runSessionTransition(
      "会话已重建，未确认的消息发送已取消",
      startAgentSession,
    );
  }, [runSessionTransition]);

  const loadSession = useCallback(
    (sessionKey: string): Promise<void> => {
      if (activeSessionKeyRef.current !== sessionKey) {
        clearLiveSessionState();
      }
      return loadAgentSession(sessionKey, lifecycleContext).then(() => undefined);
    },
    [activeSessionKeyRef, clearLiveSessionState, lifecycleContext],
  );

  const clearSession = useCallback((): void => {
    runSessionTransition(
      "会话已清空，未确认的消息发送已取消",
      clearAgentSession,
    );
  }, [runSessionTransition]);

  const bindSessionKey = useCallback(
    (key: string | null): void => {
      const normalizedKey = key?.trim() || null;
      if (activeSessionKeyRef.current === normalizedKey) {
        return;
      }

      activeSessionKeyRef.current = normalizedKey;
      // 普通会话切换不取消已发送请求；ACK registry 跨切换按
      // client_request_id 完成原 Promise，Feed 仍只显示当前会话事件。
      clearLiveSessionState();
      resetHistoryPagination();
      setSessionKey((currentKey) => (
        currentKey === normalizedKey ? currentKey : normalizedKey
      ));
      if (normalizedKey) {
        return;
      }

      setIsSessionLoading(false);
      resetRuntimeMachine();
    },
    [
      activeSessionKeyRef,
      clearLiveSessionState,
      resetHistoryPagination,
      resetRuntimeMachine,
      setIsSessionLoading,
      setSessionKey,
    ],
  );

  const resetSession = useCallback((): void => {
    runSessionTransition(
      "会话已重置，未确认的消息发送已取消",
      resetAgentSession,
    );
  }, [runSessionTransition]);

  return {
    bindSessionKey,
    clearSession,
    loadSession,
    resetSession,
    startSession,
  };
}
