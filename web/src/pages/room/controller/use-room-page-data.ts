"use client";

/**
 * INPUT: 当前路由 roomId 与 owner 代次。
 * OUTPUT: 绑定 roomId 的 Room 上下文、加载态、快照写回与刷新命令。
 * POS: Room 页面上下文资源；回访的 Room 先用本 owner 最近一次成功快照渲染，再在后台刷新。
 */
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
import { captureAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import type { RoomContextAggregate } from "@/types/conversation/room";

interface UseRoomPageDataOptions {
  roomId?: string | null;
}

interface RoomPageDataState {
  scopeKey: string;
  isRoomLoading: boolean;
  // 本次进入后服务端读取是否已确认当前快照；缓存起步的快照在读取失败时必须丢弃，不得把可能已删除的 Room 留在页面上。
  isVerified: boolean;
  roomContexts: RoomContextAggregate[];
}

// 切换 Room 时若只能从空状态起步，页面会整体退回全屏加载、拆掉 Header 与 Composer 再重建，
// 形成一帧空白。进程内保留每个 Room 最近一次成功的上下文，回访时直接渲染并后台刷新；
// owner 代次变化即整表作废，不跨账号复用。
const roomContextsCache = {
  entries: new Map<string, RoomContextAggregate[]>(),
  generation: captureAuthOwnerScopeGeneration(),
};

function roomContextsCacheEntries(): Map<string, RoomContextAggregate[]> {
  const generation = captureAuthOwnerScopeGeneration();
  if (roomContextsCache.generation !== generation) {
    roomContextsCache.entries.clear();
    roomContextsCache.generation = generation;
  }
  return roomContextsCache.entries;
}

function createRoomPageDataState(scopeKey: string): RoomPageDataState {
  const cached = scopeKey ? roomContextsCacheEntries().get(scopeKey) : undefined;
  return {
    scopeKey,
    isRoomLoading: Boolean(scopeKey) && !cached,
    isVerified: !cached,
    roomContexts: cached ?? [],
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
  return createRoomPageDataState(scopeKey);
}

function failRoomRequest(
  current: RoomPageDataState,
  scopeKey: string,
): RoomPageDataState {
  if (current.scopeKey !== scopeKey) {
    return current;
  }
  if (current.isVerified) {
    return { ...current, isRoomLoading: false };
  }
  roomContextsCacheEntries().delete(scopeKey);
  return { scopeKey, isRoomLoading: false, isVerified: false, roomContexts: [] };
}

export function useRoomPageData({roomId}: UseRoomPageDataOptions) {
  const scopeKey = roomId?.trim() ?? "";
  const scopeRef = useRef(scopeKey);
  const requestSequenceRef = useRef(0);
  const [state, setState] = useState<RoomPageDataState>(() => (
    createRoomPageDataState(scopeKey)
  ));
  scopeRef.current = scopeKey;

  // 只缓存服务端确认过的当前 scope 快照（含其后的快照写回）；加载中、缓存起步或读取失败的状态不写入。
  useEffect(() => {
    if (state.scopeKey && !state.isRoomLoading && state.isVerified) {
      roomContextsCacheEntries().set(state.scopeKey, state.roomContexts);
    }
  }, [state]);

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
          isVerified: true,
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
        setState((current) => failRoomRequest(current, scopeKey));
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
    : createRoomPageDataState(scopeKey);

  return {
    roomContexts: currentState.roomContexts,
    setRoomContexts,
    isRoomLoading: currentState.isRoomLoading,
    isRoomVerified: currentState.isVerified,
    refreshRoomContexts,
  };
}
