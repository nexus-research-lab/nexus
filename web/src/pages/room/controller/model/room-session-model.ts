import { buildRoomAgentSessionKey, buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type {
  AgentConversationChatType,
  AgentConversationIdentity,
} from "@/types/agent/agent-conversation";
import type { RoomContextAggregate } from "@/types/conversation/room";

interface ResolveRoomSessionIdentityOptions {
  currentRoomId: string | null;
  currentConversationId: string | null;
  activeRoomSession: RoomContextAggregate["sessions"][number] | null;
  currentRoomType: string;
}

interface RoomSessionKeyContext {
  agentId: string | null;
  conversationId: string;
}

type RoomSessionKeyBuilderMap = Record<
  AgentConversationChatType,
  (context: RoomSessionKeyContext) => string
>;

const ROOM_SESSION_KEY_BUILDERS = {
  dm: ({agentId, conversationId}) => agentId
    ? buildRoomAgentSessionKey(conversationId, agentId, "dm")
    : buildRoomSharedSessionKey(conversationId),
  group: ({conversationId}) => buildRoomSharedSessionKey(conversationId),
} satisfies RoomSessionKeyBuilderMap;

export function resolveCurrentAgentSessionIdentity({
  currentRoomId,
  currentConversationId,
  activeRoomSession,
  currentRoomType,
}: ResolveRoomSessionIdentityOptions): AgentConversationIdentity | null {
  const agentId = activeRoomSession?.agent_id ?? null;
  const conversationId = resolveConversationId(
    currentConversationId,
    activeRoomSession,
  );
  if (!conversationId) {
    return null;
  }

  const chatType = resolveRoomChatType(currentRoomType);

  return {
    agent_id: agentId,
    chat_type: chatType,
    conversation_id: conversationId,
    room_session_id: activeRoomSession?.id ?? null,
    room_id: currentRoomId,
    session_key: ROOM_SESSION_KEY_BUILDERS[chatType]({
      agentId,
      conversationId,
    }),
  };
}

function resolveConversationId(
  currentConversationId: string | null,
  activeRoomSession: ResolveRoomSessionIdentityOptions["activeRoomSession"],
): string | null {
  return currentConversationId ?? activeRoomSession?.conversation_id ?? null;
}

function resolveRoomChatType(roomType: string): AgentConversationChatType {
  return roomType === "dm" ? "dm" : "group";
}
