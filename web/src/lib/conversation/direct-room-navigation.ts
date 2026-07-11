import { AppRouteBuilders } from "@/app/router/route-paths";
import { ensureDirectRoom } from "@/lib/api/room-api";
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
