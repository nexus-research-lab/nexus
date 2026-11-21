import { getMessageHistoryRoundPageSize } from "@/config/conversation-policy";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { ConversationMessagesQuery } from "@/types/conversation/history";

// 后端 around 半径上限为 3；跳转与向后扩展都取满，一次请求覆盖 7 个 round。
const ROUND_WINDOW_RADIUS = 3;

export interface AgentConversationHistoryCursor {
  before_round_id: string | null;
  before_round_timestamp: number | null;
  /** 已确认的会话首个 round；它仍驻留时窗口之前不再有历史。 */
  start_round_id?: string | null;
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
  /** 当前驻留窗口内按时间排序的 round id。 */
  residentRoundIds: string[];
}

interface WindowEdgeRequestContext extends ConversationHistoryRequestContext {
  gapRoundId: string | null;
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

/** 驻留窗口之前是否还有历史：已确认的会话起点仍驻留时没有。 */
export function hasOlderHistory(
  startRoundId: string | null | undefined,
  hasMore: boolean,
  residentRoundIds: readonly string[],
): boolean {
  return startRoundId ? !residentRoundIds.includes(startRoundId) : hasMore;
}

/** 窗口 round key 对应的服务端游标 id；没有 round_id 的消息以 message_id 作为游标。 */
function historyCursorRoundId(roundKey: string): string {
  return roundKey.startsWith("message:")
    ? roundKey.slice("message:".length)
    : roundKey;
}

/**
 * 服务端游标只描述它返回的那一页之前的位置。窗口起点因淘汰或跳转移动后，
 * 游标不再指向驻留的首个 round，改用该 round 作为 id 游标读取与窗口相邻的一页。
 */
export function planOlderHistoryRequest(
  context: OlderHistoryRequestContext,
): ConversationHistoryRequest | null {
  const { cursor, residentRoundIds } = context;
  if (
    context.isLoading
    || !hasOlderHistory(cursor.start_round_id, context.hasMore, residentRoundIds)
  ) {
    return null;
  }
  const headRoundId = residentRoundIds[0]
    ? historyCursorRoundId(residentRoundIds[0])
    : null;
  if (headRoundId && headRoundId !== cursor.before_round_id) {
    return createHistoryRequest(context, {
      before_round_id: headRoundId,
      limit: getMessageHistoryRoundPageSize(),
    });
  }
  if (!cursor.before_round_timestamp) {
    return null;
  }
  return createHistoryRequest(context, {
    before_round_id: cursor.before_round_id,
    before_round_timestamp: cursor.before_round_timestamp,
    limit: getMessageHistoryRoundPageSize(),
  });
}

export function planNewerHistoryRequest(
  context: WindowEdgeRequestContext,
): ConversationHistoryRequest | null {
  const gapRoundId = context.gapRoundId?.trim() ?? "";
  if (context.isLoading || !gapRoundId) {
    return null;
  }
  return createHistoryRequest(context, {
    around_limit: ROUND_WINDOW_RADIUS,
    around_round_id: gapRoundId,
  });
}

export function planLatestHistoryRequest(
  context: WindowEdgeRequestContext,
): ConversationHistoryRequest | null {
  if (context.isLoading || !context.gapRoundId) {
    return null;
  }
  return createHistoryRequest(context, {
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
    around_limit: ROUND_WINDOW_RADIUS,
    around_round_id: roundId,
  });
}
