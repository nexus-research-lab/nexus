/**
 * INPUT: 可取消的 Launcher bootstrap loader 与目录刷新/订阅命令。
 * OUTPUT: 区分首次失败、合法空目录和 stale 数据的单飞目录状态机。
 * POS: Home 目录的纯状态与竞态边界；不订阅浏览器事件，也不拥有 React 生命周期。
 */
import type {
  LauncherAgentSummary,
  LauncherBootstrapResponse,
  LauncherConversationSummary,
  LauncherRoomSummary,
} from "@/types/app/launcher";

export interface HomeDirectorySnapshot {
  agents: LauncherAgentSummary[];
  conversations: LauncherConversationSummary[];
  hasError: boolean;
  hasLoaded: boolean;
  isLoading: boolean;
  rooms: LauncherRoomSummary[];
}

interface HomeDirectoryStoreOptions {
  load: (signal: AbortSignal) => Promise<LauncherBootstrapResponse>;
  reportError: (error: unknown) => void;
}

interface ActiveDirectoryRequest {
  controller: AbortController;
}

type DirectoryListener = () => void;

export interface HomeDirectoryStore {
  getLastSuccessfulRefreshAt: () => number;
  getSnapshot: () => HomeDirectorySnapshot;
  hasActiveRequest: () => boolean;
  refresh: () => void;
  subscribe: (listener: DirectoryListener) => () => void;
}

export function createHomeDirectoryStore({
  load,
  reportError,
}: HomeDirectoryStoreOptions): HomeDirectoryStore {
  const listeners = new Set<DirectoryListener>();
  let activeRequest: ActiveDirectoryRequest | null = null;
  let lastSuccessfulRefreshAt = 0;
  let refreshQueued = false;
  let snapshot: HomeDirectorySnapshot = {
    agents: [],
    conversations: [],
    hasError: false,
    hasLoaded: false,
    isLoading: true,
    rooms: [],
  };

  const replaceSnapshot = (nextSnapshot: HomeDirectorySnapshot) => {
    if (snapshot === nextSnapshot) {
      return;
    }
    snapshot = nextSnapshot;
    for (const listener of listeners) {
      listener();
    }
  };

  const cancelActiveRequest = () => {
    const request = activeRequest;
    refreshQueued = false;
    if (!request) {
      return;
    }
    activeRequest = null;
    request.controller.abort();
  };

  const refresh = () => {
    if (activeRequest) {
      refreshQueued = true;
      return;
    }

    const request: ActiveDirectoryRequest = {
      controller: new AbortController(),
    };
    activeRequest = request;
    replaceSnapshot({
      ...snapshot,
      hasError: false,
      isLoading: !snapshot.hasLoaded,
    });

    void load(request.controller.signal)
      .then((payload) => {
        if (activeRequest !== request) {
          return;
        }
        lastSuccessfulRefreshAt = Date.now();
        replaceSnapshot({
          agents: payload.agents,
          conversations: payload.conversations,
          hasError: false,
          hasLoaded: true,
          isLoading: false,
          rooms: payload.rooms,
        });
      })
      .catch((error: unknown) => {
        if (activeRequest !== request) {
          return;
        }
        if (isAbortError(error, request.controller.signal)) {
          replaceSnapshot({ ...snapshot, isLoading: false });
          return;
        }
        reportError(error);
        replaceSnapshot({ ...snapshot, hasError: true, isLoading: false });
      })
      .finally(() => {
        if (activeRequest === request) {
          activeRequest = null;
          if (refreshQueued) {
            refreshQueued = false;
            refresh();
          }
        }
      });
  };

  return {
    getLastSuccessfulRefreshAt: () => lastSuccessfulRefreshAt,
    getSnapshot: () => snapshot,
    hasActiveRequest: () => activeRequest !== null,
    refresh,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) {
          cancelActiveRequest();
        }
      };
    },
  };
}

function isAbortError(error: unknown, signal: AbortSignal): boolean {
  if (signal.aborted) {
    return true;
  }
  return Boolean(
    error
    && typeof error === "object"
    && "name" in error
    && error.name === "AbortError",
  );
}
