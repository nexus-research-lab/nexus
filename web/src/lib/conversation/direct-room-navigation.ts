import { AppRouteBuilders } from "@/app/router/route-paths";
import { ensure_direct_room } from "@/lib/api/room-api";
import type { RoomContextAggregate } from "@/types/conversation/room";

export interface DirectRoomNavigationTarget {
  context: RoomContextAggregate;
  route: string;
}

/**
 * 中文注释：标准化「打开某个 agent 的 DM」入口。
 * 无论来自 Launcher、侧边栏 header 还是其他入口，都必须先确保 direct room 存在，
 * 然后统一落到真实的 room_conversation 路由，避免再维护中转页。
 */
export async function resolve_direct_room_navigation_target(
  agentId: string,
  initialMessage?: string,
): Promise<DirectRoomNavigationTarget> {
  const context = await ensure_direct_room(agentId);
  const normalizedInitialMessage = initialMessage?.trim() ?? "";
  const baseRoute = AppRouteBuilders.room_conversation(
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
