/**
 * INPUT: 可取消的 Launcher bootstrap loader、目录本地增量、刷新/订阅与 owner reset 命令。
 * OUTPUT: 区分首次失败、合法空目录和 stale 数据，并隔离旧 owner 响应及晚到 HTTP 快照、按目录版本跳过重复读取的单飞状态机。
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
  revision: number;
}

type DirectoryListener = () => void;

export interface DirectoryRoomUpdate {
  roomId: string;
  conversationId?: string;
  sessionKey?: string;
  timestamp: number;
  preview?: string;
  deleted?: boolean;
}

interface RoomPatch extends DirectoryRoomUpdate {
  revision: number;
  replyTimestamp: number;
}

export interface HomeDirectoryStore {
  applyRoomUpdate: (update: DirectoryRoomUpdate) => boolean;
  getRevision: () => number;
  acceptAuthoritativePayload: (
    payload: LauncherBootstrapResponse,
    requestRevision?: number,
  ) => HomeDirectorySnapshot;
  getLastSuccessfulRefreshAt: () => number;
  getSnapshot: () => HomeDirectorySnapshot;
  hasActiveRequest: () => boolean;
  /** 传入目录版本时，已加载或在途结果不早于该版本则不再发起读取。 */
  refresh: (directoryRevision?: number) => void;
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
  // undefined 表示排队的刷新必须无条件执行；数值表示只需覆盖到该目录版本。
  let queuedDirectoryRevision: number | undefined;
  let loadedDirectoryRevision = Number.NEGATIVE_INFINITY;
  let snapshot = createInitialSnapshot();
  let revision = 0;
  let acceptedRevision = 0;
  const patches = new Map<string, RoomPatch>();

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
    queuedDirectoryRevision = undefined;
    if (!request) {
      return;
    }
    activeRequest = null;
    request.controller.abort();
  };

  const replaceWithPayload = (
    payload: LauncherBootstrapResponse,
    requestRevision: number,
  ): HomeDirectorySnapshot => {
    if (requestRevision < acceptedRevision) return snapshot;
    acceptedRevision = requestRevision;
    lastSuccessfulRefreshAt = Date.now();
    loadedDirectoryRevision = payload.directory_revision ?? Number.NEGATIVE_INFINITY;
    const nextSnapshot: HomeDirectorySnapshot = {
      agents: payload.agents,
      conversations: payload.conversations,
      hasError: false,
      hasLoaded: true,
      isLoading: false,
      rooms: payload.rooms,
    };
    let merged = nextSnapshot;
    for (const [roomId, patch] of patches) {
      if (patch.revision > requestRevision) merged = applyPatch(merged, patch);
      else patches.delete(roomId);
    }
    replaceSnapshot(merged);
    return merged;
  };

  const acceptAuthoritativePayload = (
    payload: LauncherBootstrapResponse,
    requestRevision = revision,
  ): HomeDirectorySnapshot => {
    // 显式对账读取比更早启动的被动刷新更新；取消旧请求，避免旧快照回写。
    cancelActiveRequest();
    return replaceWithPayload(payload, requestRevision);
  };

  const resetOwnerScope = () => {
    // 身份切换必须先中止旧 owner 的请求，再发布空快照；迟到响应由 request identity 栅栏丢弃。
    cancelActiveRequest();
    lastSuccessfulRefreshAt = 0;
    patches.clear();
    revision = 0;
    acceptedRevision = 0;
    loadedDirectoryRevision = Number.NEGATIVE_INFINITY;
    replaceSnapshot(createInitialSnapshot());
  };

  const refresh = (directoryRevision?: number) => {
    if (directoryRevision !== undefined && loadedDirectoryRevision >= directoryRevision) {
      return;
    }
    if (activeRequest) {
      // 在途请求的版本未知，排队等它完成后再按版本判定是否仍需重读。
      queuedDirectoryRevision = !refreshQueued
        ? directoryRevision
        : queuedDirectoryRevision === undefined || directoryRevision === undefined
          ? undefined
          : Math.max(queuedDirectoryRevision, directoryRevision);
      refreshQueued = true;
      return;
    }

    const request: ActiveDirectoryRequest = {
      controller: new AbortController(),
      revision,
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
        replaceWithPayload(payload, request.revision);
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
            const queuedRevision = queuedDirectoryRevision;
            refreshQueued = false;
            queuedDirectoryRevision = undefined;
            refresh(queuedRevision);
          }
        }
      });
  };

  const applyRoomUpdate = (update: DirectoryRoomUpdate): boolean => {
    const known = snapshot.rooms.some((room) => room.id === update.roomId)
      && snapshot.conversations.some((item) => item.room_id === update.roomId
        && (!update.conversationId || item.conversation_id === update.conversationId));
    const previous = patches.get(update.roomId);
    if (previous?.deleted && !update.deleted) return true;
    const newerActivity = update.deleted || !previous || update.timestamp >= previous.timestamp;
    const newerReply = update.preview !== undefined
      && (!previous || update.timestamp >= previous.replyTimestamp);
    const patch: RoomPatch = {
      ...(previous ?? update),
      ...(newerActivity ? update : {}),
      roomId: update.roomId,
      revision: ++revision,
      preview: newerReply ? update.preview : previous?.preview,
      replyTimestamp: newerReply ? update.timestamp : previous?.replyTimestamp ?? 0,
    };
    patches.set(update.roomId, patch);
    replaceSnapshot(applyPatch(snapshot, patch));
    return known;
  };

  return {
    applyRoomUpdate,
    getRevision: () => revision,
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

// 只覆盖事件拥有的字段，HTTP 仍负责成员、标题和新建会话身份。
function applyPatch(snapshot: HomeDirectorySnapshot, patch: RoomPatch): HomeDirectorySnapshot {
  if (patch.deleted) return {
    ...snapshot,
    rooms: snapshot.rooms.filter((room) => room.id !== patch.roomId),
    conversations: snapshot.conversations.filter((item) => item.room_id !== patch.roomId),
  };
  return {
    ...snapshot,
    conversations: snapshot.conversations.map((item) => {
      if (item.room_id !== patch.roomId) return item;
      const matches = patch.conversationId
        ? item.conversation_id === patch.conversationId
        : item.session_key === patch.sessionKey;
      return {
        ...item,
        ...(patch.preview !== undefined ? { last_reply_preview: patch.preview } : {}),
        ...(matches && patch.timestamp > (Date.parse(item.last_activity) || 0)
          ? { last_activity: new Date(patch.timestamp).toISOString() } : {}),
      };
    }),
  };
}
