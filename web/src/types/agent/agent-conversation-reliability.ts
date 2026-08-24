/**
 * INPUT: Conversation transport、请求与 Agent round 的可靠性语义。
 * OUTPUT: 前端状态机、DM/Room 面板共用的稳定可靠性类型。
 * POS: types 层的 Conversation reliability 公共契约。
 */
import type { ConversationFailureCode } from "@/types/generated/protocol";

export type { ConversationFailureCode } from "@/types/generated/protocol";

export type ConversationTransportPhase =
  | "connecting"
  | "healthy"
  | "recovering"
  | "unavailable";

export interface ConversationFailure {
  code: ConversationFailureCode;
  agent_round_id?: string | null;
  client_request_id?: string | null;
  round_id?: string | null;
  session_key: string;
}

export interface ConversationProviderRetry {
  agent_round_id?: string | null;
  round_id: string;
  session_key: string;
}

export interface ConversationReliabilitySnapshot {
  failure: ConversationFailure | null;
  provider_retry: ConversationProviderRetry | null;
  transport_phase: ConversationTransportPhase;
}

export type ConversationRecoveryEvidence =
  | {
      kind: "request_accepted";
      client_request_id: string;
      session_key?: string | null;
    }
  | {
      kind: "round_progress";
      agent_round_id?: string | null;
      round_id: string;
      session_key: string;
    }
  | {
      kind: "session_reconciled";
      failure: ConversationFailure | null;
      session_key: string;
    }
  | {
      kind: "submission_started";
      session_key: string;
    };
