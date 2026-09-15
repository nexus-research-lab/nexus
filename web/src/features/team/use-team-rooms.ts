// INPUT: 当前远程认证 owner scope 与 Relay Room 目录。
// OUTPUT: 在线 Room 列表、可用性和显式刷新动作。
// POS: 聊天侧栏消费的在线 Room 目录资源；本地免登录身份不得触达 Relay。

import { useEffect, useState, useSyncExternalStore } from "react";
import { useTeamRefresh } from "./use-team-refresh";

import { listTeamRooms, type TeamRoomView } from "@/lib/api/conversation/team-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { isRemoteAccountAuthenticated, useAuth } from "@/shared/auth/auth-context";

export function useTeamRooms() {
  const { status } = useAuth();
  const canUseRelay = isRemoteAccountAuthenticated(status);
  const [rooms, setRooms] = useState<TeamRoomView[]>([]);
  const [isAvailable, setIsAvailable] = useState(false);
  const generation = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  useEffect(() => {
    setRooms([]);
    setIsAvailable(false);
  }, [canUseRelay, generation]);
  const refresh = useTeamRefresh(canUseRelay ? String(generation) : null, async (signal) => {
    try {
      const value = await listTeamRooms(signal);
      if (!signal.aborted && isAuthOwnerScopeGenerationCurrent(generation)) {
        setRooms(value.rooms);
        setIsAvailable(true);
      }
    } catch (error) {
      if (signal.aborted || !isAuthOwnerScopeGenerationCurrent(generation)) return;
      if (error instanceof ApiRequestError && [401, 403, 404].includes(error.status)) {
        setRooms([]);
        setIsAvailable(false);
      }
      if (!(error instanceof ApiRequestError && error.status === 404)) {
        console.warn("Team Room directory failed", error);
      }
    }
  });

  return {
    isAvailable: canUseRelay && isAvailable,
    refresh,
    rooms: canUseRelay ? rooms : [],
  };
}
