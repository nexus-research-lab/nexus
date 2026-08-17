import { useSyncExternalStore } from "react";

import { getLauncherBootstrapApi } from "@/lib/api/launcher-api";
import { subscribeRoomDirectoryUpdates } from "@/lib/conversation/room-directory-events";
import { AGENT_LIST_UPDATED_EVENT_NAME } from "@/store/agent";
import type {
  LauncherAgentSummary,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

const DIRECTORY_PASSIVE_REFRESH_STALE_MS = 120_000;

export interface HomeDirectorySnapshot {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  hasError: boolean;
  isLoading: boolean;
  rooms: LauncherRoomSummary[];
}

type DirectoryListener = () => void;

const listeners = new Set<DirectoryListener>();
let snapshot: HomeDirectorySnapshot = {
  agents: [],
  conversations: [],
  hasError: false,
  isLoading: true,
  rooms: [],
};
let refreshPromise: Promise<void> | null = null;
let refreshQueued = false;
let lastSuccessfulRefreshAt = 0;
let stopTriggers: (() => void) | null = null;

export function useHomeDirectory(): HomeDirectorySnapshot {
  return useSyncExternalStore(
    subscribeHomeDirectory,
    getHomeDirectorySnapshot,
    getHomeDirectorySnapshot,
  );
}

export function refreshHomeDirectory(): void {
  if (refreshPromise) {
    // 目录事件可能发生在当前请求期间；排队一次可避免旧响应成为最终快照。
    refreshQueued = true;
    return;
  }
  if (snapshot.agents.length === 0 && snapshot.rooms.length === 0) {
    replaceSnapshot({ ...snapshot, hasError: false, isLoading: true });
  }

  refreshPromise = getLauncherBootstrapApi()
    .then((payload) => {
      lastSuccessfulRefreshAt = Date.now();
      replaceSnapshot({
        agents: payload.agents,
        conversations: payload.conversations,
        hasError: false,
        isLoading: false,
        rooms: payload.rooms,
      });
    })
    .catch((error) => {
      console.error("[HomeDirectory] 加载聊天目录失败:", error);
      replaceSnapshot({ ...snapshot, hasError: true, isLoading: false });
    })
    .finally(() => {
      refreshPromise = null;
      if (refreshQueued) {
        refreshQueued = false;
        refreshHomeDirectory();
      }
    });
}

function refreshHomeDirectoryIfStale(): void {
  if (
    refreshPromise ||
    Date.now() - lastSuccessfulRefreshAt < DIRECTORY_PASSIVE_REFRESH_STALE_MS
  ) {
    return;
  }
  refreshHomeDirectory();
}

function subscribeHomeDirectory(listener: DirectoryListener): () => void {
  listeners.add(listener);
  if (listeners.size === 1) {
    stopTriggers = startDirectoryTriggers();
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) {
      stopTriggers?.();
      stopTriggers = null;
    }
  };
}

function getHomeDirectorySnapshot(): HomeDirectorySnapshot {
  return snapshot;
}

function replaceSnapshot(nextSnapshot: HomeDirectorySnapshot): void {
  if (snapshot === nextSnapshot) {
    return;
  }
  snapshot = nextSnapshot;
  for (const listener of listeners) {
    listener();
  }
}

function startDirectoryTriggers(): () => void {
  if (!refreshPromise) {
    refreshHomeDirectory();
  }
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
