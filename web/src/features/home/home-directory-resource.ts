/**
 * INPUT: Launcher bootstrap API、Room/Agent 目录失效事件与 React 订阅生命周期。
 * OUTPUT: 全局共享、可重试且保留最后成功数据的 Home 目录快照。
 * POS: Home/Launcher/通知共用的目录资源装配层；请求状态机归 home-directory-store。
 */
import { useSyncExternalStore } from "react";

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
