/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：direct-room-navigation.ts
# @Date   ：2026-04-15 22:23
# @Author ：leemysw
# 2026-04-15 22:23   Create
# =====================================================
*/

import { AppRouteBuilders } from "@/app/router/route-paths";
import { ensureDirectRoom } from "@/lib/room-api";
import type { RoomContextAggregate } from "@/types/room";

export interface DirectRoomNavigationTarget {
  context: RoomContextAggregate;
  route: string;
}

/**
 * 中文注释：标准化「打开某个 agent 的 DM」入口。
 * 无论来自 Launcher、侧边栏 header 还是其他入口，都必须先确保 direct room 存在，
 * 然后统一落到真实的 room_conversation 路由，避免再维护中转页。
 */
export async function resolveDirectRoomNavigationTarget(
  agent_id: string,
  initial_message?: string,
): Promise<DirectRoomNavigationTarget> {
  const context = await ensureDirectRoom(agent_id);
  const normalized_initial_message = initial_message?.trim() ?? "";
  const base_route = AppRouteBuilders.room_conversation(
    context.room.id,
    context.conversation.id,
  );

  return {
    context,
    route: normalized_initial_message
      ? `${base_route}?initial=${encodeURIComponent(normalized_initial_message)}`
      : base_route,
  };
}
