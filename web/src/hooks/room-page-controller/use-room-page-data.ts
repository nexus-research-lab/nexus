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
import { getRoomContexts } from "@/lib/api/room-api";
import { RoomContextAggregate } from "@/types/conversation/room";

interface UseRoomPageDataOptions {
  roomId?: string | null;
}

interface RoomPageDataState {
  isRoomLoading: boolean;
  roomContexts: RoomContextAggregate[];
  roomError: string | null;
}

export function useRoomPageData({
  roomId: roomId,
}: UseRoomPageDataOptions) {
  const [state, setState] = useResettableState<RoomPageDataState>(
    {
      isRoomLoading: Boolean(roomId),
      roomContexts: [],
      roomError: null,
    },
    roomId ?? "",
  );
  const { isRoomLoading: isRoomLoading, roomContexts: roomContexts, roomError: roomError } = state;
  const setRoomContexts: Dispatch<SetStateAction<RoomContextAggregate[]>> = useCallback(
    (nextContexts) => {
      setState((current) => ({
        ...current,
        roomContexts: typeof nextContexts === "function"
          ? nextContexts(current.roomContexts)
          : nextContexts,
      }));
    },
    [setState],
  );

  const loadRoomContexts = useCallback(async (nextRoomId: string): Promise<RoomContextAggregate[]> => {
    return getRoomContexts(nextRoomId);
  }, []);

  const refreshRoomContexts = useCallback(async (nextRoomId: string) => {
    const contexts = await loadRoomContexts(nextRoomId);
    setState((current) => ({ ...current, roomContexts: contexts }));
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

        setState((current) => ({ ...current, roomContexts: contexts }));
      } catch (error) {
        if (cancelled) {
          return;
        }

        setState((current) => ({
          ...current,
          roomContexts: [],
          roomError: error instanceof Error ? error.message : "加载 room 失败",
        }));
      } finally {
        if (!cancelled) {
          setState((current) => ({ ...current, isRoomLoading: false }));
        }
      }
    };

    void loadRoomContext();

    return () => {
      cancelled = true;
    };
  }, [loadRoomContexts, roomId, setState]);

  return {
    isBootstrapped: true,
    roomContexts: roomContexts,
    setRoomContexts: setRoomContexts,
    roomError: roomError,
    isRoomLoading: isRoomLoading,
    refreshRoomContexts: refreshRoomContexts,
  };
}
