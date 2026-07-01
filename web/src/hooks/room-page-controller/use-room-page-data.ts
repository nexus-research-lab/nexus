/**
 * =====================================================
 * @File   ：use-room-page-data.ts
 * @Date   ：2026-04-08 11:42:07
 * @Author ：leemysw
 * 2026-04-08 11:42:07   Create
 * =====================================================
 */

"use client";

import { useCallback, useEffect, type Dispatch, type SetStateAction } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { get_room_contexts } from "@/lib/api/room-api";
import { RoomContextAggregate } from "@/types/conversation/room";

interface UseRoomPageDataOptions {
  room_id?: string | null;
}

interface RoomPageDataState {
  is_room_loading: boolean;
  room_contexts: RoomContextAggregate[];
  room_error: string | null;
}

export function useRoomPageData({
  room_id,
}: UseRoomPageDataOptions) {
  const [state, set_state] = useResettableState<RoomPageDataState>(
    {
      is_room_loading: Boolean(room_id),
      room_contexts: [],
      room_error: null,
    },
    room_id ?? "",
  );
  const { is_room_loading, room_contexts, room_error } = state;
  const set_room_contexts: Dispatch<SetStateAction<RoomContextAggregate[]>> = useCallback(
    (next_contexts) => {
      set_state((current) => ({
        ...current,
        room_contexts: typeof next_contexts === "function"
          ? next_contexts(current.room_contexts)
          : next_contexts,
      }));
    },
    [set_state],
  );

  const load_room_contexts = useCallback(async (next_room_id: string): Promise<RoomContextAggregate[]> => {
    return get_room_contexts(next_room_id);
  }, []);

  const refresh_room_contexts = useCallback(async (next_room_id: string) => {
    const contexts = await load_room_contexts(next_room_id);
    set_state((current) => ({ ...current, room_contexts: contexts }));
    return contexts;
  }, [load_room_contexts, set_state]);

  useEffect(() => {
    if (!room_id) {
      return;
    }

    let cancelled = false;

    const load_room_context = async () => {
      try {
        const contexts = await load_room_contexts(room_id);

        if (cancelled) {
          return;
        }

        set_state((current) => ({ ...current, room_contexts: contexts }));
      } catch (error) {
        if (cancelled) {
          return;
        }

        set_state((current) => ({
          ...current,
          room_contexts: [],
          room_error: error instanceof Error ? error.message : "加载 room 失败",
        }));
      } finally {
        if (!cancelled) {
          set_state((current) => ({ ...current, is_room_loading: false }));
        }
      }
    };

    void load_room_context();

    return () => {
      cancelled = true;
    };
  }, [load_room_contexts, room_id, set_state]);

  return {
    is_bootstrapped: true,
    room_contexts,
    set_room_contexts,
    room_error,
    is_room_loading,
    refresh_room_contexts,
  };
}
