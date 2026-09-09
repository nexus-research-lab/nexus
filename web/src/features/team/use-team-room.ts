import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import {
  bootstrapTeam,
  buildTeamStreamUrl,
  getTeamDifference,
  getTeamSnapshot,
  postTeamMessage,
  type TeamBootstrap,
  type TeamMessage,
} from "@/lib/api/conversation/team-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { useWebSocket } from "@/lib/websocket/use-socket";
import {
  captureAuthOwnerScopeGeneration,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";

import { parseTeamStreamEvent } from "./team-stream-event";

export function useTeamRoom() {
  const [bootstrap, setBootstrap] = useState<TeamBootstrap | null>(null);
  const [messages, setMessages] = useState<TeamMessage[]>([]);
  const [error, setError] = useState<"load" | "send" | "sync" | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSending, setIsSending] = useState(false);
  const bootstrapRef = useRef<TeamBootstrap | null>(null);
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
    value: TeamBootstrap,
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
    const requestID = ++reloadRequestRef.current;
    const value = await bootstrapTeam(signal);
    const snapshot = await loadSnapshot(value, signal);
    if (signal?.aborted || requestID !== reloadRequestRef.current) {
      return false;
    }
    bootstrapRef.current = value;
    cursorRef.current = snapshot.snapshotSeq;
    pendingHighWaterRef.current = snapshot.snapshotSeq;
    replaceMessages(snapshot.messages);
    setBootstrap(value);
    setError(null);
    return true;
  }, [loadSnapshot, replaceMessages]);

  const synchronize = useCallback(async (highWaterSeq: number) => {
    pendingHighWaterRef.current = Math.max(pendingHighWaterRef.current, highWaterSeq);
    if (syncingRef.current) {
      return;
    }
    syncingRef.current = true;
    try {
      while (cursorRef.current < pendingHighWaterRef.current) {
        const value = bootstrapRef.current;
        if (!value) {
          return;
        }
        try {
          const difference = await getTeamDifference(
            value.conversation.sync_stream_id,
            cursorRef.current,
            value.conversation.stream_epoch,
          );
          if (bootstrapRef.current !== value) {
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
    const controller = new AbortController();
    recoveringRef.current = false;
    bootstrapRef.current = null;
    cursorRef.current = 0;
    pendingHighWaterRef.current = 0;
    replaceMessages([]);
    setBootstrap(null);
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
      reloadRequestRef.current += 1;
    };
  }, [ownerGeneration, reload, replaceMessages]);

  const recoverStream = useCallback(async () => {
    if (recoveringRef.current) {
      return;
    }
    recoveringRef.current = true;
    bootstrapRef.current = null;
    cursorRef.current = 0;
    pendingHighWaterRef.current = 0;
    setBootstrap(null);
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
    const value = bootstrapRef.current;
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
    autoConnect: Boolean(bootstrap),
    heartbeatInterval: 0,
    heartbeatTimeout: 0,
    onMessage: handleStreamMessage,
    reconnect: true,
    url: bootstrap
      ? buildTeamStreamUrl(
          bootstrap.conversation.sync_stream_id,
          bootstrap.conversation.stream_epoch,
        )
      : "",
  });

  const send = useCallback(async (text: string) => {
    const value = bootstrapRef.current;
    const normalized = text.trim();
    if (!value || !normalized || sendingRef.current) {
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
  }, [replaceMessages, synchronize]);

  return { bootstrap, error, isLoading, isSending, messages, reload, send };
}

function mergeTeamMessages(messages: TeamMessage[]): TeamMessage[] {
  return [...new Map(messages.map((message) => [message.id, message])).values()]
    .sort((left, right) => left.message_seq - right.message_seq);
}
