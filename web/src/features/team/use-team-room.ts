// INPUT: Owner scope, Team resources and explicit retry/send commands.
// OUTPUT: Team read model with cancellable load retry, separate from message sending.
// POS: Team resource lifecycle owner.
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
  getTeamSnapshot,
  postTeamMessage,
  listTeamRooms,
  type TeamMessage,
  type TeamRoomView,
} from "@/lib/api/conversation/team-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { useWebSocket } from "@/lib/websocket/use-socket";
import {
  captureAuthOwnerScopeGeneration,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { isRemoteAccountAuthenticated, useAuth } from "@/shared/auth/auth-context";

import { parseTeamStreamEvent } from "./team-stream-event";

export function useTeamRoom(roomId: string | null) {
  const { status } = useAuth();
  const canUseRelay = isRemoteAccountAuthenticated(status);
  const [room, setRoom] = useState<TeamRoomView | null>(null);
  const [messages, setMessages] = useState<TeamMessage[]>([]);
  const [error, setError] = useState<"load" | "send" | "sync" | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSending, setIsSending] = useState(false);
  const retryControllerRef = useRef<AbortController | null>(null);
  const roomRef = useRef<TeamRoomView | null>(null);
  const cursorRef = useRef(0);
  const messagesRef = useRef<TeamMessage[]>([]);
  const syncingRef = useRef(false);
  const recoveringRef = useRef(false);
  const reloadRequestRef = useRef(0);
  const sendingRef = useRef(false);
  const pendingHighWaterRef = useRef(0);
  const pendingSendRef = useRef<{ id: string; text: string } | null>(null);
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );

  const replaceMessages = useCallback((next: TeamMessage[]) => {
    const merged = mergeTeamMessages(next);
    messagesRef.current = merged;
    setMessages(merged);
  }, []);

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
    const value = roomId
      ? directory.rooms.find((candidate) => candidate.room.id === roomId)
      : directory.rooms[0];
    if (!value) {
      throw new Error("Team Room 不存在");
    }
    const snapshot = await loadSnapshot(value, signal);
    if (signal?.aborted || requestID !== reloadRequestRef.current) {
      return false;
    }
    roomRef.current = value;
    cursorRef.current = snapshot.snapshotSeq;
    pendingHighWaterRef.current = snapshot.snapshotSeq;
    replaceMessages(snapshot.messages);
    setRoom(value);
    setError(null);
    return true;
  }, [canUseRelay, loadSnapshot, replaceMessages, roomId]);

  const retryLoad = useCallback(async () => {
    if (retryControllerRef.current) return;
    const controller = new AbortController();
    retryControllerRef.current = controller;
    setIsLoading(true);
    try {
      await reload(controller.signal);
    } catch {
      if (!controller.signal.aborted) setError("load");
    } finally {
      if (retryControllerRef.current === controller) {
        retryControllerRef.current = null;
        setIsLoading(false);
      }
    }
  }, [reload]);

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
          if (roomRef.current !== value) {
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
    void synchronize(event.high_water_seq).catch(() => {
      setError("sync");
    });
  }, [recoverStream, synchronize]);
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

  const send = useCallback(async (text: string) => {
    const value = roomRef.current;
    const normalized = text.trim();
    if (!canUseRelay || !value || !normalized || sendingRef.current) {
      return false;
    }
    sendingRef.current = true;
    setIsSending(true);
    setError(null);
    const command = pendingSendRef.current?.text === normalized
      ? pendingSendRef.current
      : { id: crypto.randomUUID(), text: normalized };
    pendingSendRef.current = command;
    try {
      const commit = await postTeamMessage(
        value.conversation.id,
        normalized,
        command.id,
      );
      pendingSendRef.current = null;
      replaceMessages([...messagesRef.current, commit.message]);
      try {
        await synchronize(commit.high_water_seq);
      } catch {
        setError("sync");
      }
      return true;
    } catch {
      setError("send");
      return false;
    } finally {
      sendingRef.current = false;
      setIsSending(false);
    }
  }, [canUseRelay, replaceMessages, synchronize]);

  return {
    error: canUseRelay ? error : null,
    isLoading: canUseRelay && isLoading,
    isSending: canUseRelay && isSending,
    messages: canUseRelay ? messages : [],
    reload,
    retryLoad,
    room: canUseRelay ? room : null,
    send,
  };
}

function mergeTeamMessages(messages: TeamMessage[]): TeamMessage[] {
  return [...new Map(messages.map((message) => [message.id, message])).values()]
    .sort((left, right) => left.message_seq - right.message_seq);
}
