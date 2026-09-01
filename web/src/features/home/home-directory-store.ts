/**
 * INPUT: 可取消的 Launcher bootstrap loader、目录刷新/订阅与 owner reset 命令。
 * OUTPUT: 区分首次失败、合法空目录和 stale 数据，并栅栏旧 owner 响应的单飞状态机。
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
  acceptAuthoritativePayload: (
    payload: LauncherBootstrapResponse,
  ) => HomeDirectorySnapshot;
  getLastSuccessfulRefreshAt: () => number;
  getSnapshot: () => HomeDirectorySnapshot;
  hasActiveRequest: () => boolean;
  refresh: () => void;
  resetOwnerScope: () => void;
  subscribe: (listener: DirectoryListener) => () => void;
}

function createInitialSnapshot(): HomeDirectorySnapshot {
  return {
    agents: [],
    conversations: [],
    hasError: false,
    hasLoaded: false,
    isLoading: true,
    rooms: [],
  };
}

export function createHomeDirectoryStore({
  load,
  reportError,
}: HomeDirectoryStoreOptions): HomeDirectoryStore {
  const listeners = new Set<DirectoryListener>();
  let activeRequest: ActiveDirectoryRequest | null = null;
  let lastSuccessfulRefreshAt = 0;
  let refreshQueued = false;
  let snapshot = createInitialSnapshot();

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

  const replaceWithPayload = (
    payload: LauncherBootstrapResponse,
  ): HomeDirectorySnapshot => {
    lastSuccessfulRefreshAt = Date.now();
    const nextSnapshot: HomeDirectorySnapshot = {
      agents: payload.agents,
      conversations: payload.conversations,
      hasError: false,
      hasLoaded: true,
      isLoading: false,
      rooms: payload.rooms,
    };
    replaceSnapshot(nextSnapshot);
    return nextSnapshot;
  };

  const acceptAuthoritativePayload = (
    payload: LauncherBootstrapResponse,
  ): HomeDirectorySnapshot => {
    // 显式对账读取比更早启动的被动刷新更新；取消旧请求，避免旧快照回写。
    cancelActiveRequest();
    return replaceWithPayload(payload);
  };

  const resetOwnerScope = () => {
    // 身份切换必须先中止旧 owner 的请求，再发布空快照；迟到响应由 request identity 栅栏丢弃。
    cancelActiveRequest();
    lastSuccessfulRefreshAt = 0;
    replaceSnapshot(createInitialSnapshot());
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
        replaceWithPayload(payload);
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
    acceptAuthoritativePayload,
    getLastSuccessfulRefreshAt: () => lastSuccessfulRefreshAt,
    getSnapshot: () => snapshot,
    hasActiveRequest: () => activeRequest !== null,
    refresh,
    resetOwnerScope,
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
