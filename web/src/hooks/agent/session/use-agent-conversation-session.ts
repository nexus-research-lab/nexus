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
  cancelPendingRequestAcks: (
    reason: string,
    keepPreserved?: boolean,
  ) => void;
  clearLiveSessionState: () => void;
  lifecycleContext: AgentConversationLifecycleContext;
  resetHistoryPagination: () => void;
  resetRuntimeMachine: () => void;
  setIsSessionLoading: Dispatch<SetStateAction<boolean>>;
  setSessionKey: Dispatch<SetStateAction<string | null>>;
}

type SessionTransition = (context: AgentConversationLifecycleContext) => void;
type SessionTransitionKind = "clear" | "reset" | "start";

interface SessionTransitionEffects {
  cancelPendingRequestAcks: (
    reason: string,
    keepPreserved?: boolean,
  ) => void;
  clearLiveSessionState: () => void;
  resetHistoryPagination: () => void;
  resetRuntimeMachine: () => void;
}

/**
 * 新会话只保留具备 durable transport owner 的请求；普通页面级 ACK 仍取消。
 * clear/reset 是用户明确撤销全部未确认请求的边界。
 */
export function runAgentSessionTransition(
  kind: SessionTransitionKind,
  reason: string,
  transition: SessionTransition,
  context: AgentConversationLifecycleContext,
  effects: SessionTransitionEffects,
): void {
  // start 会取消普通 chat/interrupt 等页面级 ACK，只保留显式标记为
  // durable owner 的 Goal；clear/reset 是用户明确取消全部请求的边界。
  effects.cancelPendingRequestAcks(reason, kind === "start");
  effects.clearLiveSessionState();
  transition(context);
  effects.resetHistoryPagination();
  effects.resetRuntimeMachine();
}

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
    (
      kind: SessionTransitionKind,
      reason: string,
      transition: SessionTransition,
    ): void => {
      runAgentSessionTransition(kind, reason, transition, lifecycleContext, {
        cancelPendingRequestAcks,
        clearLiveSessionState,
        resetHistoryPagination,
        resetRuntimeMachine,
      });
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
      "start",
      "已切换到新会话",
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
      "clear",
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
      "reset",
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
