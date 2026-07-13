import { getMessageHistoryRoundPageSize } from "@/config/conversation-policy";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { ConversationMessagesQuery } from "@/types/conversation/history";

const TARGET_ROUND_WINDOW_RADIUS = 1;

export interface AgentConversationHistoryCursor {
  before_round_id: string | null;
  before_round_timestamp: number | null;
}

type ConversationHistorySource =
  | {
      kind: "room";
      conversationId: string;
      roomId: string;
    }
  | {
      kind: "session";
      sessionKey: string;
    };

export interface ConversationHistoryRequest {
  activeSessionKey: string;
  query: ConversationMessagesQuery;
  source: ConversationHistorySource;
}

interface ConversationHistoryRequestContext {
  activeSessionKey: string | null;
  identity: AgentConversationIdentity | null;
}

interface OlderHistoryRequestContext extends ConversationHistoryRequestContext {
  cursor: AgentConversationHistoryCursor;
  hasMore: boolean;
  isLoading: boolean;
}

interface RoundWindowRequestContext extends ConversationHistoryRequestContext {
  isLoading: boolean;
  roundId: string;
}

function normalizeIdentityValue(value: string | null | undefined): string {
  return value?.trim() ?? "";
}

function resolveRoomHistorySource(
  identity: AgentConversationIdentity | null,
): ConversationHistorySource | null {
  const roomId = normalizeIdentityValue(identity?.room_id);
  const conversationId = normalizeIdentityValue(identity?.conversation_id);
  if (!roomId || !conversationId) {
    return null;
  }
  return { kind: "room", conversationId, roomId };
}

function resolveHistorySource(
  activeSessionKey: string,
  identity: AgentConversationIdentity | null,
): ConversationHistorySource {
  return resolveRoomHistorySource(identity)
    ?? { kind: "session", sessionKey: activeSessionKey };
}

function createHistoryRequest(
  context: ConversationHistoryRequestContext,
  query: ConversationMessagesQuery,
): ConversationHistoryRequest | null {
  const activeSessionKey = context.activeSessionKey;
  if (!activeSessionKey) {
    return null;
  }
  return {
    activeSessionKey,
    query,
    source: resolveHistorySource(activeSessionKey, context.identity),
  };
}

export function planOlderHistoryRequest(
  context: OlderHistoryRequestContext,
): ConversationHistoryRequest | null {
  const canLoad = context.hasMore
    && !context.isLoading
    && Boolean(context.cursor.before_round_timestamp);
  if (!canLoad) {
    return null;
  }
  return createHistoryRequest(context, {
    before_round_id: context.cursor.before_round_id,
    before_round_timestamp: context.cursor.before_round_timestamp,
    limit: getMessageHistoryRoundPageSize(),
  });
}

export function planRoundWindowHistoryRequest(
  context: RoundWindowRequestContext,
): ConversationHistoryRequest | null {
  const roundId = context.roundId.trim();
  if (context.isLoading || !roundId) {
    return null;
  }
  return createHistoryRequest(context, {
    around_limit: TARGET_ROUND_WINDOW_RADIUS,
    around_round_id: roundId,
  });
}
