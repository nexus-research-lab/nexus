import { AppRouteBuilders } from "@/app/router/route-paths";
import { ensureDirectRoom } from "@/lib/api/room-api";
import type { RoomContextAggregate } from "@/types/conversation/room";

export interface DirectRoomNavigationTarget {
  context: RoomContextAggregate;
  route: string;
}

/**
 * 中文注释：标准化「打开某个 agent 的 DM」入口。
 * 无论来自 Launcher、侧边栏 header 还是其他入口，都必须先确保 direct room 存在，
 * 然后统一落到真实的 roomConversation 路由，避免再维护中转页。
 */
export async function resolveDirectRoomNavigationTarget(
  agentId: string,
  initialMessage?: string,
): Promise<DirectRoomNavigationTarget> {
  const context = await ensureDirectRoom(agentId);
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
