"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import type { RoomContextAggregate } from "@/types/conversation/room";

interface UseRoomPageDataOptions {
  roomId?: string | null;
}

interface RoomPageDataState {
  scopeKey: string;
  isRoomLoading: boolean;
  roomContexts: RoomContextAggregate[];
}

function createRoomPageDataState(scopeKey: string, isLoading: boolean): RoomPageDataState {
  return {
    scopeKey,
    isRoomLoading: isLoading,
    roomContexts: [],
  };
}

function isActiveRoomScope(
  scopeRef: RefObject<string>,
  scopeKey: string,
): boolean {
  return Boolean(scopeKey) && scopeRef.current === scopeKey;
}

function isCurrentRoomRequest(
  scopeRef: RefObject<string>,
  requestSequenceRef: RefObject<number>,
  scopeKey: string,
  requestId: number,
): boolean {
  return scopeRef.current === scopeKey
    && requestSequenceRef.current === requestId;
}

function beginRoomRequest(
  current: RoomPageDataState,
  scopeKey: string,
): RoomPageDataState {
  if (current.scopeKey === scopeKey) {
    return current;
  }
  return createRoomPageDataState(scopeKey, true);
}

function finishRoomRequest(
  current: RoomPageDataState,
  scopeKey: string,
): RoomPageDataState {
  return current.scopeKey === scopeKey
    ? { ...current, isRoomLoading: false }
    : current;
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
    if (!isActiveRoomScope(scopeRef, scopeKey)) {
      return [];
    }

    const requestId = ++requestSequenceRef.current;
    setState((current) => beginRoomRequest(current, scopeKey));

    try {
      const contexts = await getRoomContexts(scopeKey);
      if (isCurrentRoomRequest(
        scopeRef,
        requestSequenceRef,
        scopeKey,
        requestId,
      )) {
        setState({
          scopeKey,
          isRoomLoading: false,
          roomContexts: contexts,
        });
      }
      return contexts;
    } catch (error) {
      if (isCurrentRoomRequest(
        scopeRef,
        requestSequenceRef,
        scopeKey,
        requestId,
      )) {
        setState((current) => finishRoomRequest(current, scopeKey));
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
    roomContexts: currentState.roomContexts,
    setRoomContexts,
    isRoomLoading: currentState.isRoomLoading,
    refreshRoomContexts,
  };
}
