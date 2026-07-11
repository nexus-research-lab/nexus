import { AppRouteBuilders } from "@/app/router/route-paths";
import { ensureDirectRoom } from "@/lib/api/conversation/room-command-api";
import { ApiRequestError } from "@/lib/api/core/http";
import { useAgentStore } from "@/store/agent";
import type { RoomContextAggregate } from "@/types/conversation/room";

export interface DirectRoomNavigationTarget {
  context: RoomContextAggregate;
  route: string;
}

/**
 * 所有 DM 入口先解析真实 Direct Room，再进入 Conversation 路由，避免各入口复制创建协议。
 */
export async function resolveDirectRoomNavigationTarget(
  agentId: string,
  initialMessage?: string,
): Promise<DirectRoomNavigationTarget> {
  const context = await resolveDirectRoom(agentId);
  const normalizedInitialMessage = initialMessage?.trim() ?? "";
  const baseRoute = AppRouteBuilders.roomConversation(
    context.room.id,
    context.conversation.id,
  );

  return {
    context,
    route: normalizedInitialMessage
      ? `${baseRoute}?initial=${encodeURIComponent(normalizedInitialMessage)}`
      : baseRoute,
  };
}

async function resolveDirectRoom(agentId: string): Promise<RoomContextAggregate> {
  try {
    return await ensureDirectRoom(agentId);
  } catch (error) {
    if (error instanceof ApiRequestError && error.status === 404) {
      const agentStore = useAgentStore.getState();
      if (agentStore.current_agent_id === agentId) {
        agentStore.set_current_agent(null);
      }
      void agentStore.load_agents_from_server();
    }
    throw error;
  }
}
