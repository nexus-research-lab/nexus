// INPUT: 账号作用域、Team 资源、成员新快照与显式读取/发送动作。
// OUTPUT: 可取消的读取恢复、同步快照与冻结意图的消息重试。
// POS: Team 聊天资源生命周期所有者；元数据刷新不重载消息历史。
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import {
  buildTeamStreamUrl,
  getTeamDifference,
	getTeamRoom,
  getTeamSnapshot,
  postTeamMessage,
  listTeamRooms,
  type TeamMessage,
  type TeamRoomView,
	type TeamRoomDetails,
} from "@/lib/api/conversation/team-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { useWebSocket } from "@/lib/websocket/use-socket";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { isRemoteAccountAuthenticated, useAuth } from "@/shared/auth/auth-context";

import { parseTeamStreamEvent } from "./team-stream-event";
import { useTeamRefresh } from "./use-team-refresh";
import { TeamMessageOutbox, type TeamMessageIntent } from "./team-message-outbox";
import { isTeamCommandUnapplied } from "./team-command-outcome";

export function useTeamRoom(roomId: string | null) {
  const { status } = useAuth();
  const canUseRelay = isRemoteAccountAuthenticated(status);
  const [room, setRoom] = useState<TeamRoomDetails | null>(null);
  const [messages, setMessages] = useState<TeamMessage[]>([]);
  const [error, setError] = useState<"load" | "send" | "sync" | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSending, setIsSending] = useState(false);
  const [hasUnconfirmedSend, setHasUnconfirmedSend] = useState(false);
  const [pendingText, setPendingText] = useState<string | null>(null);
  const retryControllerRef = useRef<AbortController | null>(null);
  const roomRef = useRef<TeamRoomDetails | null>(null);
  const cursorRef = useRef(0);
  const messagesRef = useRef<TeamMessage[]>([]);
  const syncingRef = useRef(false);
  const recoveringRef = useRef(false);
  const reloadRequestRef = useRef(0);
  const sendingRef = useRef(false);
  const pendingHighWaterRef = useRef(0);
  const pendingSendRef = useRef<TeamMessageIntent | null>(null);
  const outboxRef = useRef<TeamMessageOutbox | null>(null);
  const controlUserId = status?.control_user_id ?? status?.user_id;
  const organizationId = status?.organization_id;
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );

  const restorePending = useCallback(() => {
    const pending = outboxRef.current?.read()[0] ?? null;
    pendingSendRef.current = pending;
    setHasUnconfirmedSend(Boolean(pending));
    setPendingText(pending?.text ?? null);
  }, []);

  const replaceMessages = useCallback((next: TeamMessage[]) => {
    const merged = mergeTeamMessages(next);
    for (const message of merged) {
      if (message.author_type === "user" && message.author_user_id === controlUserId) outboxRef.current?.confirm(message.client_message_id);
    }
    restorePending();
    messagesRef.current = merged;
    setMessages(merged);
  }, [controlUserId, restorePending]);

  const handleReadFailure = useCallback((cause: unknown) => {
    if (cause instanceof ApiRequestError && [401, 403, 404].includes(cause.status)) {
      // 明确撤权不能保留连接与可发送的旧快照，私有未确认意图仍保留待后续对账。
      roomRef.current = null;
      reloadRequestRef.current += 1;
      setRoom(null);
      messagesRef.current = [];
      setMessages([]);
      setError("load");
      return;
    }
    setError("sync");
  }, []);

  const updateDetails = useCallback((next: TeamRoomDetails) => {
    const current = roomRef.current;
    if (!current || current.room.id !== next.room.id ||
      current.room.membership_version > next.room.membership_version ||
      current.room.configuration_version > next.room.configuration_version) return;
    roomRef.current = next;
    setRoom(next);
  }, []);

  const refreshDetails = useCallback(async (signal?: AbortSignal) => {
    const generation = captureAuthOwnerScopeGeneration();
    const current = roomRef.current;
    if (!current) return;
    try {
      const next = await getTeamRoom(current.room.id, signal);
      if (!signal?.aborted && isAuthOwnerScopeGenerationCurrent(generation)) updateDetails(next);
    } catch (cause) {
      if (!signal?.aborted && isAuthOwnerScopeGenerationCurrent(generation)) handleReadFailure(cause);
    }
  }, [handleReadFailure, updateDetails]);
  useTeamRefresh(canUseRelay && room ? `${ownerGeneration}:${room.room.id}` : null, async (signal) => {
    try { await refreshDetails(signal); }
    catch { if (!signal.aborted) setError("sync"); }
  });

  const loadSnapshot = useCallback(async (
    value: TeamRoomView,
    signal?: AbortSignal,
  ): Promise<{ messages: TeamMessage[]; snapshotSeq: number }> => {
    let afterMessageSeq = 0;
    let throughMessageSeq: number | null = null;
    let snapshotSeq: number | null = null;
    const loaded: TeamMessage[] = [];
    for (;;) {
      const query = new URLSearchParams({
        after_message_seq: String(afterMessageSeq),
        limit: "100",
      });
      if (throughMessageSeq !== null && snapshotSeq !== null) {
        query.set("through_message_seq", String(throughMessageSeq));
        query.set("snapshot_seq", String(snapshotSeq));
        query.set("stream_epoch", value.conversation.stream_epoch);
      }
      const page = await getTeamSnapshot(value.conversation.id, query, signal);
      loaded.push(...page.messages);
      throughMessageSeq = page.through_message_seq;
      snapshotSeq = page.snapshot_seq;
      if (!page.has_more) {
        return { messages: loaded, snapshotSeq: page.snapshot_seq };
      }
      afterMessageSeq = page.next_message_seq;
    }
  }, []);

  const reload = useCallback(async (signal?: AbortSignal) => {
    if (!canUseRelay) {
      return false;
    }
    const requestID = ++reloadRequestRef.current;
    const directory = await listTeamRooms(signal);
    const listed = roomId
      ? directory.rooms.find((candidate) => candidate.room.id === roomId)
      : directory.rooms[0];
    if (!listed) {
      throw new ApiRequestError("Team Room 不存在", 404);
    }
	const value = await getTeamRoom(listed.room.id, signal);
    const snapshot = await loadSnapshot(value, signal);
    if (signal?.aborted || requestID !== reloadRequestRef.current) {
      return false;
    }
    roomRef.current = value;
    outboxRef.current = controlUserId && organizationId
      ? new TeamMessageOutbox(JSON.stringify([organizationId, controlUserId, value.conversation.id]))
      : null;
    cursorRef.current = snapshot.snapshotSeq;
    pendingHighWaterRef.current = snapshot.snapshotSeq;
    replaceMessages(snapshot.messages);
    setRoom(value);
    setError(null);
    return true;
  }, [canUseRelay, controlUserId, organizationId, loadSnapshot, replaceMessages, roomId]);

  const retryLoad = useCallback(async () => {
    if (retryControllerRef.current) return;
    const controller = new AbortController();
    retryControllerRef.current = controller;
    setIsLoading(true);
    try {
      await reload(controller.signal);
    } catch (cause) {
      if (!controller.signal.aborted) { handleReadFailure(cause); setError("load"); }
    } finally {
      if (retryControllerRef.current === controller) {
        retryControllerRef.current = null;
        setIsLoading(false);
      }
    }
  }, [handleReadFailure, reload]);

  const synchronize = useCallback(async (highWaterSeq: number) => {
    pendingHighWaterRef.current = Math.max(pendingHighWaterRef.current, highWaterSeq);
    if (syncingRef.current) {
      return;
    }
    syncingRef.current = true;
    try {
      while (cursorRef.current < pendingHighWaterRef.current) {
        const value = roomRef.current;
        if (!value) {
          return;
        }
        try {
          const difference = await getTeamDifference(
            value.conversation.sync_stream_id,
            cursorRef.current,
            value.conversation.stream_epoch,
          );
          if (roomRef.current?.conversation.id !== value.conversation.id || roomRef.current.conversation.stream_epoch !== value.conversation.stream_epoch) {
            return;
          }
          replaceMessages([
            ...messagesRef.current,
            ...difference.events.map((event) => event.message),
          ]);
          if (difference.next_seq <= cursorRef.current) {
            return;
          }
          cursorRef.current = difference.next_seq;
          pendingHighWaterRef.current = Math.max(
            pendingHighWaterRef.current,
            difference.high_water_seq,
          );
        } catch (cause) {
          if (cause instanceof ApiRequestError && cause.failure?.code === "team.full_snapshot_required") {
            if (!await reload()) {
              return;
            }
            continue;
          }
          throw cause;
        }
      }
    } finally {
      syncingRef.current = false;
    }
  }, [reload, replaceMessages]);

  useEffect(() => {
    pendingSendRef.current = null;
    outboxRef.current = null;
    setHasUnconfirmedSend(false);
    setPendingText(null);
    if (!canUseRelay) {
      roomRef.current = null;
      replaceMessages([]);
      setRoom(null);
      setError(null);
      setIsLoading(false);
      return;
    }
    const controller = new AbortController();
    recoveringRef.current = false;
    roomRef.current = null;
    cursorRef.current = 0;
    pendingHighWaterRef.current = 0;
    replaceMessages([]);
    setRoom(null);
    setError(null);
    setIsLoading(true);
    void reload(controller.signal)
      .catch(() => {
        if (!controller.signal.aborted) {
          setError("load");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setIsLoading(false);
        }
      });
    return () => {
      controller.abort();
      retryControllerRef.current?.abort();
      retryControllerRef.current = null;
      reloadRequestRef.current += 1;
    };
  }, [canUseRelay, ownerGeneration, reload, replaceMessages]);

  const recoverStream = useCallback(async () => {
    if (recoveringRef.current) {
      return;
    }
    recoveringRef.current = true;
    roomRef.current = null;
    cursorRef.current = 0;
    pendingHighWaterRef.current = 0;
    setRoom(null);
    replaceMessages([]);
    setIsLoading(true);
    try {
      await reload();
    } catch {
      setError("load");
    } finally {
      recoveringRef.current = false;
      setIsLoading(false);
    }
  }, [reload, replaceMessages]);

  const handleStreamMessage = useCallback((message: unknown) => {
    const value = roomRef.current;
    const event = value ? parseTeamStreamEvent(message, value) : null;
    if (!event) {
      return;
    }
    if (event.type === "stream.reset_required") {
      void recoverStream();
      return;
    }
    void synchronize(event.high_water_seq).catch(handleReadFailure);
  }, [handleReadFailure, recoverStream, synchronize]);
  useWebSocket({
    autoConnect: canUseRelay && Boolean(room),
    heartbeatInterval: 0,
    heartbeatTimeout: 0,
    onMessage: handleStreamMessage,
    reconnect: true,
    url: room
      ? buildTeamStreamUrl(
          room.conversation.sync_stream_id,
          room.conversation.stream_epoch,
        )
      : "",
  });

  const send = useCallback(async (text: string, agentIds: string[] = []) => {
    const generation = captureAuthOwnerScopeGeneration();
    const value = roomRef.current;
    const normalized = (pendingSendRef.current?.text ?? text).trim();
    if (!canUseRelay || !value || !normalized || sendingRef.current) {
      return false;
    }
    sendingRef.current = true;
    setIsSending(true);
    setError(null);
	const normalizedAgentIDs = [...new Set(agentIds)];
    // 未确认的写入必须重放原始意图，不能在刷新成员后替换版本或静默丢弃目标。
    const previous = pendingSendRef.current;
    const command = previous ?? { agentIds: normalizedAgentIDs, id: crypto.randomUUID(), text: normalized, membershipVersion: value.room.membership_version };
    try {
      if (!outboxRef.current) throw new Error("缺少在线账号的持久化作用域");
      outboxRef.current.save(command);
      pendingSendRef.current = command;
      setPendingText(command.text);
      setHasUnconfirmedSend(true);
      const commit = await postTeamMessage(
        value.conversation.id,
        command.text,
        command.id,
		command.agentIds.length > 0 ? { agentIds: command.agentIds, expectedMembershipVersion: command.membershipVersion } : undefined,
      );
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return false;
      outboxRef.current.confirm(command.id);
      replaceMessages([...messagesRef.current, commit.message]);
      try {
        await synchronize(commit.high_water_seq);
      } catch (cause) {
        handleReadFailure(cause);
      }
      return true;
    } catch (cause) {
      if (!isAuthOwnerScopeGenerationCurrent(generation)) return false;
      if (isTeamCommandUnapplied(cause, previous !== null)) {
        outboxRef.current?.confirm(command.id);
        pendingSendRef.current = null;
        setHasUnconfirmedSend(false);
        setPendingText(null);
      }
      await refreshDetails().catch(() => undefined);
      if (roomRef.current) setError("send");
      return false;
    } finally {
      sendingRef.current = false;
      setIsSending(false);
    }
  }, [canUseRelay, handleReadFailure, refreshDetails, replaceMessages, synchronize]);

  return {
    error: canUseRelay ? error : null,
    isLoading: canUseRelay && isLoading,
    isSending: canUseRelay && isSending,
    hasUnconfirmedSend: canUseRelay && hasUnconfirmedSend,
    pendingText: canUseRelay ? pendingText : null,
    messages: canUseRelay ? messages : [],
    retryLoad,
    room: canUseRelay ? room : null,
    send,
    updateDetails,
  };
}

function mergeTeamMessages(messages: TeamMessage[]): TeamMessage[] {
  return [...new Map(messages.map((message) => [message.id, message])).values()]
    .sort((left, right) => left.message_seq - right.message_seq);
}
