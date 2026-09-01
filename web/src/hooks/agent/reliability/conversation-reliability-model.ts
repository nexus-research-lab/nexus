/**
 * INPUT: transport 状态、结构化会话故障与带精确关联身份的恢复证据。
 * OUTPUT: 不泄露底层错误详情、可由后续正向证据撤销的 Conversation reliability 快照。
 * POS: DM/Room 共用的纯可靠性状态机；不拥有网络请求或 React 生命周期。
 */
import type { WebSocketState } from "@/types/system/websocket";
import type {
  ConversationFailure,
  ConversationProviderRetry,
  ConversationRecoveryEvidence,
  ConversationReliabilitySnapshot,
} from "@/types/agent/agent-conversation-reliability";

export interface ConversationReliabilityState
  extends ConversationReliabilitySnapshot {
  active_session_key: string | null;
  has_connected: boolean;
}

export function hasConversationReliabilityNotice(
  reliability: ConversationReliabilitySnapshot,
): boolean {
  return reliability.transport_phase === "recovering"
    || reliability.transport_phase === "unavailable"
    || reliability.provider_retry !== null
    || reliability.failure !== null;
}

export type ConversationReliabilityAction =
  | { type: "scope_changed"; session_key: string | null }
  | { type: "transport_observed"; state: WebSocketState }
  | { type: "failure_reported"; failure: ConversationFailure }
  | { type: "provider_retry_observed"; retry: ConversationProviderRetry }
  | { type: "recovery_observed"; evidence: ConversationRecoveryEvidence };

export const INITIAL_CONVERSATION_RELIABILITY_STATE: ConversationReliabilityState = {
  active_session_key: null,
  failure: null,
  has_connected: false,
  provider_retry: null,
  transport_phase: "connecting",
};

function normalized(value?: string | null): string {
  return value?.trim() ?? "";
}

function belongsToActiveSession(
  activeSessionKey: string | null,
  sessionKey: string,
): boolean {
  const normalizedSessionKey = normalized(sessionKey);
  return normalizedSessionKey.length > 0
    && normalizedSessionKey === normalized(activeSessionKey);
}

function correlationMatches(
  failure: ConversationFailure,
  evidence: Extract<ConversationRecoveryEvidence, { kind: "round_progress" }>,
): boolean {
  if (normalized(failure.agent_round_id)) {
    return normalized(failure.agent_round_id) === normalized(evidence.agent_round_id);
  }
  return normalized(failure.round_id) !== ""
    && normalized(failure.round_id) === normalized(evidence.round_id);
}

function sessionActivityDisprovesFailure(failure: ConversationFailure): boolean {
  return failure.code === "connection_unavailable"
    || failure.code === "provider_configuration"
    || failure.code === "provider_unavailable"
    || failure.code === "session_load_failed";
}

function submissionSupersedesFailure(failure: ConversationFailure): boolean {
  return failure.code === "permission_not_sent"
    || failure.code === "request_rejected"
    || failure.code === "round_failed"
    || failure.code === "safety_rejected"
    || failure.code === "validation_failed";
}

function reduceTransportState(
  state: ConversationReliabilityState,
  websocketState: WebSocketState,
): ConversationReliabilityState {
  if (websocketState === "connected") {
    return {
      ...state,
      failure: state.failure?.code === "connection_unavailable"
        ? null
        : state.failure,
      has_connected: true,
      transport_phase: "healthy",
    };
  }
  if (websocketState === "failed") {
    return {
      ...state,
      transport_phase: "unavailable",
    };
  }
  if (websocketState === "reconnecting") {
    return {
      ...state,
      transport_phase: state.has_connected || state.transport_phase === "unavailable"
        ? "recovering"
        : "connecting",
    };
  }
  return {
    ...state,
    transport_phase: state.has_connected || state.transport_phase === "unavailable"
      ? "recovering"
      : "connecting",
  };
}

function reduceRecoveryEvidence(
  state: ConversationReliabilityState,
  evidence: ConversationRecoveryEvidence,
): ConversationReliabilityState {
  if (evidence.kind === "session_reconciled") {
    if (!belongsToActiveSession(state.active_session_key, evidence.session_key)) {
      return state;
    }
    return {
      ...state,
      failure: evidence.failure,
      provider_retry: null,
    };
  }
  if (evidence.kind === "submission_started") {
    if (!belongsToActiveSession(state.active_session_key, evidence.session_key)) {
      return state;
    }
    if (!state.failure || !submissionSupersedesFailure(state.failure)) {
      return state;
    }
    return {
      ...state,
      failure: null,
    };
  }
  if (evidence.kind === "request_accepted") {
    const failure = state.failure;
    if (
      !failure
      || normalized(failure.client_request_id) === ""
      || normalized(failure.client_request_id) !== normalized(evidence.client_request_id)
      || (evidence.session_key
        && normalized(failure.session_key) !== normalized(evidence.session_key))
    ) {
      return state;
    }
    return { ...state, failure: null };
  }

  if (!belongsToActiveSession(state.active_session_key, evidence.session_key)) {
    return state;
  }
  const failure = state.failure && (
      correlationMatches(state.failure, evidence)
      || sessionActivityDisprovesFailure(state.failure)
    )
    ? null
    : state.failure;
  const providerRetry = state.provider_retry
    && correlationMatches(
      {
        code: "round_failed",
        ...state.provider_retry,
      },
      evidence,
    )
    ? null
    : state.provider_retry;
  if (failure === state.failure && providerRetry === state.provider_retry) {
    return state;
  }
  return {
    ...state,
    failure,
    provider_retry: providerRetry,
  };
}

export function reduceConversationReliabilityState(
  state: ConversationReliabilityState,
  action: ConversationReliabilityAction,
): ConversationReliabilityState {
  switch (action.type) {
    case "scope_changed": {
      const sessionKey = normalized(action.session_key) || null;
      if (sessionKey === state.active_session_key) {
        return state;
      }
      return {
        ...state,
        active_session_key: sessionKey,
        failure: null,
        provider_retry: null,
      };
    }
    case "transport_observed":
      return reduceTransportState(state, action.state);
    case "failure_reported":
      return belongsToActiveSession(
          state.active_session_key,
          action.failure.session_key,
        )
        ? { ...state, failure: action.failure }
        : state;
    case "provider_retry_observed":
      return belongsToActiveSession(
          state.active_session_key,
          action.retry.session_key,
        )
        ? { ...state, failure: null, provider_retry: action.retry }
        : state;
    case "recovery_observed":
      return reduceRecoveryEvidence(state, action.evidence);
  }
}

export function projectConversationReliabilitySnapshot(
  state: ConversationReliabilityState,
  sessionKey: string | null,
): ConversationReliabilitySnapshot {
  const isCurrent = normalized(state.active_session_key) === normalized(sessionKey);
  return {
    failure: isCurrent ? state.failure : null,
    provider_retry: isCurrent ? state.provider_retry : null,
    transport_phase: state.transport_phase,
  };
}
