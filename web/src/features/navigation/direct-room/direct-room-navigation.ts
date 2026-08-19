import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  createRoomConversation,
  ensureDirectRoom,
} from "@/lib/api/conversation/room-command-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
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
  options?: { onboarding?: boolean },
): Promise<DirectRoomNavigationTarget> {
  const directRoomContext = await resolveDirectRoom(agentId);
  const context = options?.onboarding
    ? await createRoomConversation(directRoomContext.room.id, {
        title: "Nexus 新手旅程",
      })
    : directRoomContext;
  const normalizedInitialMessage = initialMessage?.trim() ?? "";
  const baseRoute = AppRouteBuilders.roomConversation(
    context.room.id,
    context.conversation.id,
  );

  const searchParams = new URLSearchParams();
  if (normalizedInitialMessage) {
    searchParams.set("initial", normalizedInitialMessage);
  }
  if (options?.onboarding) {
    searchParams.set("onboarding", "1");
  }
  const query = searchParams.toString();

  return {
    context,
    route: query ? `${baseRoute}?${query}` : baseRoute,
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
