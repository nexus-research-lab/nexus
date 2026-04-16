/**
 * =====================================================
 * @File   ：use-room-page-data.ts
 * @Date   ：2026-04-08 11:42:07
 * @Author ：leemysw
 * 2026-04-08 11:42:07   Create
 * =====================================================
 */

"use client";

import { useCallback, useEffect, useState } from "react";

import { get_room, get_room_contexts, list_rooms, subscribe_room_list_updates } from "@/lib/api/room-api";
import { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";

interface UseRoomPageDataOptions {
  room_id?: string | null;
  load_agents_from_server: () => Promise<void>;
  load_conversations_from_server: () => Promise<void>;
}

export function useRoomPageData({
  room_id,
  load_agents_from_server,
  load_conversations_from_server,
}: UseRoomPageDataOptions) {
  const [is_bootstrapped, set_is_bootstrapped] = useState(false);
  const [room_contexts, set_room_contexts] = useState<RoomContextAggregate[]>([]);
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [is_room_loading, set_is_room_loading] = useState(false);
  const [room_error, set_room_error] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      try {
        await Promise.all([
          load_agents_from_server(),
          load_conversations_from_server(),
          list_rooms(200).then(set_rooms),
        ]);
      } finally {
        if (!cancelled) {
          set_is_bootstrapped(true);
        }
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, [load_agents_from_server, load_conversations_from_server]);

  const refresh_rooms = useCallback(async () => {
    const next_rooms = await list_rooms(200);
    set_rooms(next_rooms);
    return next_rooms;
  }, []);

  useEffect(() => subscribe_room_list_updates(() => {
    void refresh_rooms();
  }), [refresh_rooms]);

  const load_room_contexts = useCallback(async (next_room_id: string): Promise<RoomContextAggregate[]> => {
    const [room, contexts] = await Promise.all([
      get_room(next_room_id),
      get_room_contexts(next_room_id),
    ]);

    if (contexts.length) {
      return contexts;
    }

    return [
      {
        room: room.room,
        members: room.members,
        conversation: {
          id: "",
          room_id: room.room.id,
          conversation_type: "room_main",
          title: room.room.name ?? "",
        },
        sessions: [],
      },
    ];
  }, []);

  const refresh_room_contexts = useCallback(async (next_room_id: string) => {
    const contexts = await load_room_contexts(next_room_id);
    set_room_contexts(contexts);
    return contexts;
  }, [load_room_contexts]);

  useEffect(() => {
    if (!room_id) {
      set_room_contexts([]);
      set_room_error(null);
      set_is_room_loading(false);
      return;
    }

    let cancelled = false;
    set_is_room_loading(true);
    set_room_error(null);

    const load_room_context = async () => {
      try {
        const [contexts] = await Promise.all([
          load_room_contexts(room_id),
          load_conversations_from_server(),
        ]);

        if (cancelled) {
          return;
        }

        set_room_contexts(contexts);
      } catch (error) {
        if (cancelled) {
          return;
        }

        set_room_contexts([]);
        set_room_error(error instanceof Error ? error.message : "加载 room 失败");
      } finally {
        if (!cancelled) {
          set_is_room_loading(false);
        }
      }
    };

    void load_room_context();

    return () => {
      cancelled = true;
    };
  }, [load_conversations_from_server, load_room_contexts, room_id]);

  return {
    is_bootstrapped,
    room_contexts,
    set_room_contexts,
    rooms,
    room_error,
    is_room_loading,
    refresh_rooms,
    refresh_room_contexts,
  };
}
