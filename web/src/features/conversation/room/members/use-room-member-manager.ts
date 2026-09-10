// INPUT: 当前 Room、owner 代次与辅助 Agent 目录准备命令。
// OUTPUT: 单飞加载和临时弹窗状态；切 Room/owner 或卸载后丢弃迟到打开。
// POS: Room 成员入口共用生命周期；不写成员，不取消/重放目录请求。

import { useEffect, useMemo, useRef, useSyncExternalStore } from "react";

import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";

export function useRoomMemberManager(roomId: string | null, prepareCatalog: () => Promise<void>) {
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration,
  );
  const scopeKey = JSON.stringify([ownerGeneration, roomId]);
  const scope = useMemo(() => ({ scopeKey }), [scopeKey]);
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const requestRef = useRef<{ scopeKey: string } | null>(null);
  const [phase, setPhase] = useResettableState<"closed" | "loading" | "open">("closed", scopeKey);
  useEffect(() => () => { requestRef.current = null; }, [scopeKey]);

  function close() {
    if (scopeRef.current !== scope || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
    requestRef.current = null;
    setPhase("closed");
  }

  function open() {
    if (!roomId || phase === "open" || scopeRef.current !== scope
      || requestRef.current?.scopeKey === scopeKey
      || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
    const request = { scopeKey };
    requestRef.current = request;
    setPhase("loading");
    void (async () => {
      try {
        await prepareCatalog();
      } catch (error) {
        // 目录是辅助读取；保持现有 AgentStore 失败后用已有成员打开的行为。
        console.error("[RoomMemberManager] Agent catalog refresh failed:", error);
      }
      if (requestRef.current !== request || scopeRef.current !== scope
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)) return;
      requestRef.current = null;
      setPhase("open");
    })();
  }

  return { open, close, isLoading: phase === "loading", isOpen: phase === "open" };
}
