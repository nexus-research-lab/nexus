/**
 * INPUT: Launcher bootstrap API、Room/Agent 目录失效事件、owner 切换与 React 订阅生命周期。
 * OUTPUT: 全局共享、同 owner 可保留 stale、跨 owner 立即清空的 Home 目录快照。
 * POS: Home/Launcher/通知共用的目录资源装配层；请求状态机归 home-directory-store。
 */
import { useSyncExternalStore } from "react";

import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
} from "@/shared/auth/auth-owner-generation";
import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import { subscribeRoomDirectoryUpdates } from "@/lib/conversation/room-directory-events";
import { AGENT_LIST_UPDATED_EVENT_NAME } from "@/store/agent";

import {
  createHomeDirectoryStore,
  type HomeDirectorySnapshot,
} from "./home-directory-store";

const DIRECTORY_PASSIVE_REFRESH_STALE_MS = 120_000;

type DirectoryListener = () => void;

const listeners = new Set<DirectoryListener>();
let stopTriggers: (() => void) | null = null;
const directoryStore = createHomeDirectoryStore({
  load: getLauncherBootstrapApi,
  reportError: (error) => {
    console.error("[HomeDirectory] 加载聊天目录失败:", error);
  },
});

export type { HomeDirectorySnapshot } from "./home-directory-store";

export function useHomeDirectory(): HomeDirectorySnapshot {
  return useSyncExternalStore(
    subscribeHomeDirectory,
    directoryStore.getSnapshot,
    directoryStore.getSnapshot,
  );
}

export function refreshHomeDirectory(): void {
  directoryStore.refresh();
}

/** 清空旧 owner 目录并中止其在途读取；现有订阅者可在新身份下立即重新加载。 */
export function resetHomeDirectoryOwnerScope(reload: boolean): void {
  directoryStore.resetOwnerScope();
  if (reload && listeners.size > 0) {
    directoryStore.refresh();
  }
}

/** 强制读取并提交一份权威目录，供结果未知的修改按 exact Room 对账。 */
export async function reconcileHomeDirectory(): Promise<HomeDirectorySnapshot> {
  const ownerScopeGeneration = captureAuthOwnerScopeGeneration();
  const payload = await getLauncherBootstrapApi();
  if (!isAuthOwnerScopeGenerationCurrent(ownerScopeGeneration)) {
    throw new Error("Owner scope changed while reconciling the home directory");
  }
  return directoryStore.acceptAuthoritativePayload(payload);
}

function refreshHomeDirectoryIfStale(): void {
  if (
    directoryStore.hasActiveRequest() ||
    Date.now() - directoryStore.getLastSuccessfulRefreshAt()
      < DIRECTORY_PASSIVE_REFRESH_STALE_MS
  ) {
    return;
  }
  refreshHomeDirectory();
}

function subscribeHomeDirectory(listener: DirectoryListener): () => void {
  listeners.add(listener);
  const unsubscribeStore = directoryStore.subscribe(listener);
  if (listeners.size === 1) {
    stopTriggers = startDirectoryTriggers();
  }
  return () => {
    listeners.delete(listener);
    unsubscribeStore();
    if (listeners.size === 0) {
      stopTriggers?.();
      stopTriggers = null;
    }
  };
}

function startDirectoryTriggers(): () => void {
  refreshHomeDirectory();
  if (typeof window === "undefined") {
    return () => undefined;
  }

  const refreshIfVisible = () => {
    if (document.visibilityState !== "hidden") {
      refreshHomeDirectoryIfStale();
    }
  };
  const unsubscribeRoomDirectory = subscribeRoomDirectoryUpdates(refreshHomeDirectory);
  window.addEventListener("focus", refreshIfVisible);
  window.addEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refreshHomeDirectory);
  document.addEventListener("visibilitychange", refreshIfVisible);

  return () => {
    unsubscribeRoomDirectory();
    window.removeEventListener("focus", refreshIfVisible);
    window.removeEventListener(AGENT_LIST_UPDATED_EVENT_NAME, refreshHomeDirectory);
    document.removeEventListener("visibilitychange", refreshIfVisible);
  };
}
