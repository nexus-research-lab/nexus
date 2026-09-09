// INPUT: 当前远程认证 owner scope 与 Relay Room 目录。
// OUTPUT: 在线 Room 列表、可用性和显式刷新动作。
// POS: 聊天侧栏消费的在线 Room 目录资源；本地免登录身份不得触达 Relay。

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";

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
  const [revision, setRevision] = useState(0);
  const generation = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const refresh = useCallback(() => setRevision((value) => value + 1), []);

  useEffect(() => {
    if (!canUseRelay) {
      setRooms([]);
      setIsAvailable(false);
      return;
    }
    const controller = new AbortController();
    setRooms([]);
    void listTeamRooms(controller.signal).then((value) => {
      if (isAuthOwnerScopeGenerationCurrent(generation)) {
        setRooms(value.rooms);
        setIsAvailable(true);
      }
    }).catch((error: unknown) => {
      if (isAuthOwnerScopeGenerationCurrent(generation)) {
        setIsAvailable(false);
      }
      if (!(error instanceof ApiRequestError && error.status === 404)) {
        console.warn("Team Room directory failed", error);
      }
    });
    return () => controller.abort();
  }, [canUseRelay, generation, revision]);

  return {
    isAvailable: canUseRelay && isAvailable,
    refresh,
    rooms: canUseRelay ? rooms : [],
  };
}
