// INPUT: 当前远程认证 owner scope 与 Relay Room 目录。
// OUTPUT: 在线 Room 列表、可用性和显式刷新动作。
// POS: 聊天侧栏消费的在线 Room 目录资源；本地免登录身份不得触达 Relay。

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { useTeamRefresh } from "./use-team-refresh";
import { prepareTeamRooms } from "@/lib/api/conversation/team-node-api";

import { listTeamRooms, type TeamRoomView } from "@/lib/api/conversation/team-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { hasOrganizationAccess, useAuth } from "@/shared/auth/auth-context";

export function useTeamRooms() {
  const { status } = useAuth();
  const canUseRelay = hasOrganizationAccess(status);
  const [rooms, setRooms] = useState<TeamRoomView[]>([]);
  const [isAvailable, setIsAvailable] = useState(false);
  const revision = useRef(0);
  const generation = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const scope = canUseRelay ? JSON.stringify([generation, status?.organization_id, status?.control_user_id, status?.organization_role]) : null;
  const preparation = useRef("");
  const preparationKey = scope ? JSON.stringify([scope, rooms.filter((item) => !item.room.direct_user_id).map((item) => [item.room.id, item.room.membership_version])]) : null;
  useTeamRefresh(preparationKey, async (signal) => {
    if (preparation.current === preparationKey) return;
    const ids = rooms.filter((item) => !item.room.direct_user_id).map((item) => item.room.id);
    if (!ids.length) return;
    // 目录常驻，不依赖打开某个群；失败退避仅重试原准备操作，不轮询任务。
    let delay = 1000;
    while (!signal.aborted) {
      try {
        for (let offset = 0; offset < ids.length; offset += 256) await prepareTeamRooms(ids.slice(offset, offset + 256), signal);
        if (!signal.aborted) preparation.current = preparationKey ?? "";
        return;
      } catch (error) {
        if (signal.aborted) return;
        if (error instanceof ApiRequestError && [401, 403, 404].includes(error.status)) return;
        console.warn("Team Agent preparation failed", error);
        await new Promise<void>((resolve) => {
          const done = () => { clearTimeout(timer); signal.removeEventListener("abort", done); resolve(); };
          const timer = setTimeout(done, delay);
          signal.addEventListener("abort", done, {once: true});
        });
        delay = Math.min(delay * 2, 30000);
      }
    }
  });
  useEffect(() => {
    setRooms([]);
    setIsAvailable(false);
  }, [scope]);
  const refresh = useTeamRefresh(scope, async (signal) => {
    const requestedRevision = revision.current;
    try {
      const value = await listTeamRooms(signal);
      if (!signal.aborted && requestedRevision === revision.current && isAuthOwnerScopeGenerationCurrent(generation)) {
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
    remove: (roomId: string) => {
      revision.current += 1;
      setRooms((current) => current.filter((value) => value.room.id !== roomId));
    },
    rooms: canUseRelay ? rooms : [],
  };
}
