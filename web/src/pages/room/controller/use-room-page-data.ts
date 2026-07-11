"use client";

import { useCallback, useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";

import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import type { RoomContextAggregate } from "@/types/conversation/room";

interface UseRoomPageDataOptions {
  roomId?: string | null;
}

interface RoomPageDataState {
  scopeKey: string;
  isRoomLoading: boolean;
  roomContexts: RoomContextAggregate[];
  roomError: string | null;
}

function createRoomPageDataState(scopeKey: string, isLoading: boolean): RoomPageDataState {
  return {
    scopeKey,
    isRoomLoading: isLoading,
    roomContexts: [],
    roomError: null,
  };
}

export function useRoomPageData({roomId}: UseRoomPageDataOptions) {
  const scopeKey = roomId?.trim() ?? "";
  const scopeRef = useRef(scopeKey);
  const requestSequenceRef = useRef(0);
  const [state, setState] = useState<RoomPageDataState>(() => (
    createRoomPageDataState(scopeKey, Boolean(scopeKey))
  ));
  scopeRef.current = scopeKey;

  const setRoomContexts: Dispatch<SetStateAction<RoomContextAggregate[]>> = useCallback(
    (nextContexts) => {
      setState((current) => {
        if (scopeRef.current !== scopeKey || current.scopeKey !== scopeKey) {
          return current;
        }
        return {
          ...current,
          roomContexts: typeof nextContexts === "function"
            ? nextContexts(current.roomContexts)
            : nextContexts,
        };
      });
    },
    [scopeKey],
  );

  const refreshRoomContexts = useCallback(async (): Promise<RoomContextAggregate[]> => {
    if (!scopeKey || scopeRef.current !== scopeKey) {
      return [];
    }

    const requestId = ++requestSequenceRef.current;
    setState((current) => ({
      scopeKey,
      isRoomLoading: current.scopeKey === scopeKey ? current.isRoomLoading : true,
      roomContexts: current.scopeKey === scopeKey ? current.roomContexts : [],
      roomError: null,
    }));

    try {
      const contexts = await getRoomContexts(scopeKey);
      if (scopeRef.current === scopeKey && requestSequenceRef.current === requestId) {
        setState({
          scopeKey,
          isRoomLoading: false,
          roomContexts: contexts,
          roomError: null,
        });
      }
      return contexts;
    } catch (error) {
      if (scopeRef.current === scopeKey && requestSequenceRef.current === requestId) {
        setState((current) => current.scopeKey === scopeKey
          ? {
              ...current,
              isRoomLoading: false,
              roomError: error instanceof Error ? error.message : "加载 room 失败",
            }
          : current);
      }
      throw error;
    }
  }, [scopeKey]);

  useEffect(() => {
    if (scopeKey) {
      void refreshRoomContexts().catch(() => undefined);
    }
  }, [refreshRoomContexts, scopeKey]);

  const currentState = state.scopeKey === scopeKey
    ? state
    : createRoomPageDataState(scopeKey, Boolean(scopeKey));

  return {
    isBootstrapped: true,
    roomContexts: currentState.roomContexts,
    setRoomContexts,
    roomError: currentState.roomError,
    isRoomLoading: currentState.isRoomLoading,
    refreshRoomContexts,
  };
}
