/**
 * INPUT: 当前 Session、WebSocket 状态、结构化故障与恢复证据。
 * OUTPUT: DM/Room 面板使用的可靠性快照和稳定 dispatch commands。
 * POS: Conversation reliability 纯状态机的 React 生命周期适配层。
 */
import { useCallback, useMemo, useReducer } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { WebSocketState } from "@/types/system/websocket";
import type {
  ConversationFailure,
  ConversationProviderRetry,
  ConversationRecoveryEvidence,
} from "@/types/agent/agent-conversation-reliability";

import { latestAssistantResultFailure } from "../message/assistant-message-model";
import {
  INITIAL_CONVERSATION_RELIABILITY_STATE,
  projectConversationReliabilitySnapshot,
  reduceConversationReliabilityState,
} from "./conversation-reliability-model";

export interface ConversationReliabilityController {
  changeScope: (sessionKey: string | null) => void;
  observeProviderRetry: (retry: ConversationProviderRetry) => void;
  observeRecovery: (evidence: ConversationRecoveryEvidence) => void;
  observeTransport: (state: WebSocketState) => void;
  reconcileSession: (
    sessionKey: string,
    messages: readonly Message[],
    chatType: "dm" | "group",
  ) => void;
  reportFailure: (failure: ConversationFailure) => void;
}

export function useConversationReliability() {
  const [state, dispatch] = useReducer(
    reduceConversationReliabilityState,
    INITIAL_CONVERSATION_RELIABILITY_STATE,
  );

  const changeScope = useCallback((sessionKey: string | null) => {
    dispatch({ type: "scope_changed", session_key: sessionKey });
  }, []);

  const observeProviderRetry = useCallback((retry: ConversationProviderRetry) => {
    dispatch({ type: "provider_retry_observed", retry });
  }, []);
  const observeRecovery = useCallback((evidence: ConversationRecoveryEvidence) => {
    dispatch({ type: "recovery_observed", evidence });
  }, []);
  const observeTransport = useCallback((websocketState: WebSocketState) => {
    dispatch({ type: "transport_observed", state: websocketState });
  }, []);
  const reportFailure = useCallback((failure: ConversationFailure) => {
    dispatch({ type: "failure_reported", failure });
  }, []);
  const reconcileSession = useCallback((
    reconciledSessionKey: string,
    messages: readonly Message[],
    chatType: "dm" | "group",
  ) => {
    const resultFailure = chatType === "dm"
      ? latestAssistantResultFailure(messages)
      : null;
    dispatch({
      type: "recovery_observed",
      evidence: {
        failure: resultFailure
          ? {
              agent_round_id: resultFailure.agent_round_id,
              code: resultFailure.code,
              round_id: resultFailure.round_id,
              session_key: reconciledSessionKey,
            }
          : null,
        kind: "session_reconciled",
        session_key: reconciledSessionKey,
      },
    });
  }, []);

  const controller = useMemo<ConversationReliabilityController>(() => ({
    changeScope,
    observeProviderRetry,
    observeRecovery,
    observeTransport,
    reconcileSession,
    reportFailure,
  }), [
    changeScope,
    observeProviderRetry,
    observeRecovery,
    observeTransport,
    reconcileSession,
    reportFailure,
  ]);

  return {
    controller,
    projectSnapshot: (sessionKey: string | null) => (
      projectConversationReliabilitySnapshot(state, sessionKey)
    ),
  };
}
