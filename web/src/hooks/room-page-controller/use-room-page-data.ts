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
  room_id: roomId,
}: UseRoomPageDataOptions) {
  const [state, setState] = useResettableState<RoomPageDataState>(
    {
      is_room_loading: Boolean(roomId),
      room_contexts: [],
      room_error: null,
    },
    roomId ?? "",
  );
  const { is_room_loading: isRoomLoading, room_contexts: roomContexts, room_error: roomError } = state;
  const setRoomContexts: Dispatch<SetStateAction<RoomContextAggregate[]>> = useCallback(
    (nextContexts) => {
      setState((current) => ({
        ...current,
        room_contexts: typeof nextContexts === "function"
          ? nextContexts(current.room_contexts)
          : nextContexts,
      }));
    },
    [setState],
  );

  const loadRoomContexts = useCallback(async (nextRoomId: string): Promise<RoomContextAggregate[]> => {
    return get_room_contexts(nextRoomId);
  }, []);

  const refreshRoomContexts = useCallback(async (nextRoomId: string) => {
    const contexts = await loadRoomContexts(nextRoomId);
    setState((current) => ({ ...current, room_contexts: contexts }));
    return contexts;
  }, [loadRoomContexts, setState]);

  useEffect(() => {
    if (!roomId) {
      return;
    }

    let cancelled = false;

    const loadRoomContext = async () => {
      try {
        const contexts = await loadRoomContexts(roomId);

        if (cancelled) {
          return;
        }

        setState((current) => ({ ...current, room_contexts: contexts }));
      } catch (error) {
        if (cancelled) {
          return;
        }

        setState((current) => ({
          ...current,
          room_contexts: [],
          room_error: error instanceof Error ? error.message : "加载 room 失败",
        }));
      } finally {
        if (!cancelled) {
          setState((current) => ({ ...current, is_room_loading: false }));
        }
      }
    };

    void loadRoomContext();

    return () => {
      cancelled = true;
    };
  }, [loadRoomContexts, roomId, setState]);

  return {
    is_bootstrapped: true,
    room_contexts: roomContexts,
    set_room_contexts: setRoomContexts,
    room_error: roomError,
    is_room_loading: isRoomLoading,
    refresh_room_contexts: refreshRoomContexts,
  };
}
