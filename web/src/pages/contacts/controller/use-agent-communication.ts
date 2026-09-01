/**
 * INPUT: 当前详情 Agent id、好友私域记录 API 与通讯发送命令。
 * OUTPUT: Agent 视角的好友目录、Session、私信时间线、分阶段读取快照和发送事务。
 * POS: Contacts 页面好友私聊客户端的数据与命令边界。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import {
  openAgentContactChannelApi,
  sendAgentCommunicationMessageApi,
} from "@/lib/api/agent/agent-communication-api";
import {
  deleteAgentContactApi,
  listAgentContactsApi,
  upsertAgentContactApi,
} from "@/lib/api/agent/agent-api";
import {
  listAgentPrivateEventsApi,
  listAgentPrivateThreadsApi,
} from "@/lib/api/agent/private-domain-api";
import { createRoomConversation } from "@/lib/api/conversation/room-command-api";
import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { getResourceFailure } from "@/lib/error-message";
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import type { AgentContact } from "@/types/agent/agent";
import type {
  AgentCommunicationMutationFailure,
  AgentCommunicationReadFailure,
  AgentCommunicationReadFailureKind,
} from "@/types/agent/communication";
import type { AgentPrivateEvent } from "@/types/agent/private-domain";
import type { RoomContextAggregate } from "@/types/conversation/room";

import {
  blocksAgentCommunicationIntent,
  buildAgentCommunicationMutationFailure,
  reconcileContactDirectoryMutation,
} from "./agent-communication-recovery";

const MESSAGE_LIMIT = 160;
const EMPTY_HISTORY_CURSOR = {
  beforeMessageId: null,
  beforeTimestamp: null,
  threadId: null,
} as const;

interface MessageHistoryCursor {
  beforeMessageId: string | null;
  beforeTimestamp: number | null;
  threadId: string | null;
}

export interface AgentCommunicationResource {
  contacts: AgentContact[];
  conversationId: string | null;
  conversationFailure: AgentCommunicationReadFailure | null;
  directEvents: AgentPrivateEvent[];
  directoryFailure: AgentCommunicationReadFailure | null;
  hasMoreHistory: boolean;
  historyPrependToken: number;
  isDirectoryLoading: boolean;
  isHistoryLoading: boolean;
  isMessagesLoading: boolean;
  isSending: boolean;
  mutationFailure: AgentCommunicationMutationFailure | null;
  pendingAgentId: string | null;
  roomContexts: RoomContextAggregate[];
  selectedContactId: string | null;
  addContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  clearMutationFailure: () => void;
  clearSelection: () => void;
  createConversation: (title?: string) => Promise<string | null>;
  loadOlderMessages: () => Promise<boolean>;
  removeContact: (contactAgentId: string) => Promise<boolean>;
  refresh: (kind?: AgentCommunicationReadFailureKind) => void;
  selectContact: (contactAgentId: string) => void;
  selectConversation: (conversationId: string) => void;
  sendMessage: (content: string) => Promise<void>;
}

export function useAgentCommunication(
  agentId: string | null,
): AgentCommunicationResource {
  const scopeAgentId = agentId?.trim() ?? "";
  const activeAgentIdRef = useRef(scopeAgentId);
  activeAgentIdRef.current = scopeAgentId;
  const [contacts, setContacts] = useState<AgentContact[]>([]);
  const [selectedContactId, setSelectedContactId] = useState<string | null>(null);
  const [roomContexts, setRoomContexts] = useState<RoomContextAggregate[]>([]);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [directEvents, setDirectEvents] = useState<AgentPrivateEvent[]>([]);
  const [directoryFailure, setDirectoryFailure] =
    useState<AgentCommunicationReadFailure | null>(null);
  const [conversationFailure, setConversationFailure] =
    useState<AgentCommunicationReadFailure | null>(null);
  const [mutationFailure, setMutationFailure] =
    useState<AgentCommunicationMutationFailure | null>(null);
  const [isDirectoryLoading, setIsDirectoryLoading] = useState(Boolean(scopeAgentId));
  const [isMessagesLoading, setIsMessagesLoading] = useState(false);
  const [isHistoryLoading, setIsHistoryLoading] = useState(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(false);
  const [historyCursor, setHistoryCursor] = useState<MessageHistoryCursor>(EMPTY_HISTORY_CURSOR);
  const [historyPrependToken, setHistoryPrependToken] = useState(0);
  const [isSending, setIsSending] = useState(false);
  const [pendingAgentId, setPendingAgentId] = useState<string | null>(null);
  const [isRemoving, setIsRemoving] = useState(false);
  const targetKey = `${scopeAgentId}:${selectedContactId ?? "none"}`;
  const messageKey = `${targetKey}:${conversationId ?? "none"}`;
  const roomId = roomContexts[0]?.room.id ?? null;
  const historyScopeKey = `${messageKey}:${roomId ?? "none"}`;
  const activeTargetKeyRef = useRef(targetKey);
  const activeMessageKeyRef = useRef(messageKey);
  const historyLoadingRef = useRef(false);
  const directorySnapshotAgentIdRef = useRef<string | null>(null);
  const targetSnapshotKeyRef = useRef<string | null>(null);
  const messageSnapshotKeyRef = useRef<string | null>(null);
  activeTargetKeyRef.current = targetKey;
  activeMessageKeyRef.current = messageKey;

  const loadDirectory = useCallback(async () => {
    if (!scopeAgentId) {
      return;
    }
    setIsDirectoryLoading(true);
    setDirectoryFailure(null);
    try {
      const nextContacts = await listAgentContactsApi(scopeAgentId);
      if (activeAgentIdRef.current !== scopeAgentId) {
        return;
      }
      directorySnapshotAgentIdRef.current = scopeAgentId;
      setContacts(nextContacts);
      setMutationFailure((current) => reconcileContactDirectoryMutation(
        current,
        new Set(nextContacts.map((item) => item.contact_agent_id)),
      ));
      setSelectedContactId((current) => (
        current && nextContacts.some((item) => item.contact_agent_id === current)
          ? current
          : nextContacts[0]?.contact_agent_id ?? null
      ));
    } catch (loadError) {
      if (activeAgentIdRef.current === scopeAgentId) {
        const invalidated = invalidatesReadSnapshot(loadError);
        if (invalidated) {
          directorySnapshotAgentIdRef.current = null;
          targetSnapshotKeyRef.current = null;
          messageSnapshotKeyRef.current = null;
          setContacts([]);
          setSelectedContactId(null);
          setRoomContexts([]);
          setConversationId(null);
          setDirectEvents([]);
        }
        setDirectoryFailure({
          kind: "directory",
          stale: !invalidated
            && directorySnapshotAgentIdRef.current === scopeAgentId,
        });
      }
    } finally {
      if (activeAgentIdRef.current === scopeAgentId) {
        setIsDirectoryLoading(false);
      }
    }
  }, [scopeAgentId]);

  useEffect(() => {
    setContacts([]);
    setSelectedContactId(null);
    setRoomContexts([]);
    setConversationId(null);
    setDirectEvents([]);
    setDirectoryFailure(null);
    setConversationFailure(null);
    setMutationFailure(null);
    setIsDirectoryLoading(Boolean(scopeAgentId));
    setIsMessagesLoading(false);
    setIsHistoryLoading(false);
    setHasMoreHistory(false);
    setHistoryCursor(EMPTY_HISTORY_CURSOR);
    setHistoryPrependToken(0);
    historyLoadingRef.current = false;
    setIsSending(false);
    setPendingAgentId(null);
    setIsRemoving(false);
    directorySnapshotAgentIdRef.current = null;
    targetSnapshotKeyRef.current = null;
    messageSnapshotKeyRef.current = null;
    void loadDirectory();
  }, [loadDirectory, scopeAgentId]);

  const loadTarget = useCallback(async () => {
    const requestKey = targetKey;
    if (!scopeAgentId || !selectedContactId) {
      setRoomContexts([]);
      setConversationId(null);
      return;
    }
    setIsMessagesLoading(true);
    setConversationFailure(null);
    try {
      const opened = await openAgentContactChannelApi(scopeAgentId, selectedContactId);
      const loadedContexts = await getRoomContexts(opened.room.id);
      const contexts = loadedContexts.length > 0 ? loadedContexts : [opened];
      if (activeTargetKeyRef.current !== requestKey) {
        return;
      }
      targetSnapshotKeyRef.current = requestKey;
      setContacts((current) => current.map((contact) => (
        contact.contact_agent_id === selectedContactId
          ? { ...contact, direct_room_id: opened.room.id }
          : contact
      )));
      setRoomContexts(contexts);
      setConversationId((current) => (
        current && contexts.some((item) => item.conversation.id === current)
          ? current
          : contexts[0]?.conversation.id ?? null
      ));
    } catch (loadError) {
      if (activeTargetKeyRef.current === requestKey) {
        const notFound = isNotFound(loadError);
        const invalidated = notFound || invalidatesReadSnapshot(loadError);
        if (invalidated) {
          targetSnapshotKeyRef.current = null;
          messageSnapshotKeyRef.current = null;
          setRoomContexts([]);
          setConversationId(null);
          setDirectEvents([]);
        }
        setConversationFailure(notFound
          ? null
          : {
              kind: "channel",
              stale: !invalidated
                && targetSnapshotKeyRef.current === requestKey,
            });
      }
    } finally {
      if (activeTargetKeyRef.current === requestKey) {
        setIsMessagesLoading(false);
      }
    }
  }, [scopeAgentId, selectedContactId, targetKey]);

  useEffect(() => {
    targetSnapshotKeyRef.current = null;
    messageSnapshotKeyRef.current = null;
    setRoomContexts([]);
    setConversationId(null);
    setDirectEvents([]);
    setConversationFailure(null);
    void loadTarget();
  }, [loadTarget]);

  const loadMessages = useCallback(async (
    showLoading: boolean,
    replaceHistory = false,
  ) => {
    const requestKey = messageKey;
    if (!scopeAgentId || !selectedContactId || !roomId || !conversationId) {
      return;
    }
    if (showLoading) {
      setIsMessagesLoading(true);
    }
    setConversationFailure((current) => (
      current?.kind === "channel" ? current : null
    ));
    try {
      const query = {
        conversation_id: conversationId,
        limit: MESSAGE_LIMIT,
        room_id: roomId,
      };
      const page = await listAgentPrivateThreadsApi(scopeAgentId, query);
      const thread = page.items.find((item) => (
        item.peer_agent_ids.length === 1
        && item.peer_agent_ids[0] === selectedContactId
      )) ?? page.items[0];
      const eventPage = thread
        ? (await listAgentPrivateEventsApi(
          scopeAgentId,
          thread.thread_id,
          query,
        ))
        : null;
      if (activeMessageKeyRef.current === requestKey) {
        messageSnapshotKeyRef.current = requestKey;
        const events = eventPage?.items ?? [];
        setDirectEvents((current) => replaceHistory
          ? events
          : mergePrivateEvents(current, events));
        if (replaceHistory) {
          setHistoryCursor({
            beforeMessageId: eventPage?.next_before_message_id ?? null,
            beforeTimestamp: eventPage?.next_before_timestamp ?? null,
            threadId: thread?.thread_id ?? null,
          });
          setHasMoreHistory(eventPage?.has_more ?? false);
        }
      }
    } catch (loadError) {
      if (activeMessageKeyRef.current === requestKey) {
        const invalidated = invalidatesReadSnapshot(loadError);
        if (invalidated) {
          targetSnapshotKeyRef.current = null;
          messageSnapshotKeyRef.current = null;
          setRoomContexts([]);
          setConversationId(null);
          setDirectEvents([]);
          setHasMoreHistory(false);
          setHistoryCursor(EMPTY_HISTORY_CURSOR);
        }
        setConversationFailure({
          kind: "messages",
          stale: !invalidated
            && messageSnapshotKeyRef.current === requestKey,
        });
      }
    } finally {
      if (showLoading && activeMessageKeyRef.current === requestKey) {
        setIsMessagesLoading(false);
      }
    }
  }, [conversationId, messageKey, roomId, scopeAgentId, selectedContactId]);

  const loadOlderMessages = useCallback(async (): Promise<boolean> => {
    const requestKey = messageKey;
    if (!scopeAgentId || !roomId || !conversationId ||
      !historyCursor.threadId || !hasMoreHistory || historyLoadingRef.current ||
      (!historyCursor.beforeMessageId && !historyCursor.beforeTimestamp)) {
      return false;
    }
    historyLoadingRef.current = true;
    setIsHistoryLoading(true);
    setConversationFailure((current) => (
      current?.kind === "history" ? null : current
    ));
    try {
      const page = await listAgentPrivateEventsApi(
        scopeAgentId,
        historyCursor.threadId,
        {
          before_message_id: historyCursor.beforeMessageId,
          before_timestamp: historyCursor.beforeTimestamp,
          conversation_id: conversationId,
          limit: MESSAGE_LIMIT,
          room_id: roomId,
        },
      );
      if (activeMessageKeyRef.current !== requestKey) {
        return false;
      }
      messageSnapshotKeyRef.current = requestKey;
      setHasMoreHistory(page.has_more);
      setHistoryCursor({
        beforeMessageId: page.next_before_message_id ?? null,
        beforeTimestamp: page.next_before_timestamp ?? null,
        threadId: historyCursor.threadId,
      });
      setConversationFailure((current) => (
        current?.kind === "history" ? null : current
      ));
      if (page.items.length === 0) {
        return false;
      }
      setDirectEvents((current) => mergePrivateEvents(page.items, current));
      setHistoryPrependToken((current) => current + 1);
      return true;
    } catch (loadError) {
      if (activeMessageKeyRef.current === requestKey) {
        const invalidated = invalidatesReadSnapshot(loadError);
        if (invalidated) {
          targetSnapshotKeyRef.current = null;
          messageSnapshotKeyRef.current = null;
          setRoomContexts([]);
          setConversationId(null);
          setDirectEvents([]);
          setHasMoreHistory(false);
          setHistoryCursor(EMPTY_HISTORY_CURSOR);
        }
        setConversationFailure({
          kind: "history",
          stale: !invalidated
            && messageSnapshotKeyRef.current === requestKey,
        });
      }
      return false;
    } finally {
      if (activeMessageKeyRef.current === requestKey) {
        historyLoadingRef.current = false;
        setIsHistoryLoading(false);
      }
    }
  }, [
    conversationId,
    hasMoreHistory,
    historyCursor,
    messageKey,
    roomId,
    scopeAgentId,
  ]);

  useEffect(() => {
    messageSnapshotKeyRef.current = null;
    setDirectEvents([]);
    setHasMoreHistory(false);
    setHistoryCursor(EMPTY_HISTORY_CURSOR);
    setHistoryPrependToken(0);
    setIsHistoryLoading(false);
    historyLoadingRef.current = false;
    if (conversationId && roomId) {
      void loadMessages(true, true);
    }
  }, [conversationId, historyScopeKey, loadMessages, roomId]);

  const handleRealtimeMessage = useCallback((rawMessage: unknown) => {
    const event = parseEventMessage(rawMessage);
    if (!event) {
      return;
    }
    if (event.event_type === "directory_changed"
      && event.data.reason === "agent_contact_changed") {
      void loadDirectory();
      return;
    }
    if (event.room_id !== roomId || event.conversation_id !== conversationId) {
      return;
    }
    if (event.event_type === "room_directed_message"
      || event.event_type === "room_directed_message_consumed") {
      void loadMessages(false);
    }
  }, [conversationId, loadDirectory, loadMessages, roomId]);
  const { send: sendRealtime, state: realtimeState } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: Boolean(scopeAgentId),
    reconnect: true,
    heartbeatInterval: 30_000,
    onMessage: handleRealtimeMessage,
  });
  useAppEventSubscription(sendRealtime, realtimeState);

  const previousRealtimeStateRef = useRef(realtimeState);

  useEffect(() => {
    if (!roomId || realtimeState !== "connected") {
      return;
    }
    sendRealtime({
      type: "subscribe_room",
      room_id: roomId,
      ...(conversationId ? { conversation_id: conversationId } : {}),
    });
    return () => {
      sendRealtime({
        type: "unsubscribe_room",
        room_id: roomId,
        ...(conversationId ? { conversation_id: conversationId } : {}),
      });
    };
  }, [conversationId, realtimeState, roomId, sendRealtime]);

  useEffect(() => {
    const previousRealtimeState = previousRealtimeStateRef.current;
    previousRealtimeStateRef.current = realtimeState;
    if (
      realtimeState !== "connected"
      || previousRealtimeState === "connected"
    ) {
      return;
    }
    void loadMessages(false);
  }, [loadMessages, realtimeState]);

  const addContact = useCallback(async (
    contactAgentId: string,
    alias: string,
  ): Promise<boolean> => {
    const targetAgentId = contactAgentId.trim();
    if (!scopeAgentId || !targetAgentId || pendingAgentId) {
      return false;
    }
    const intentKey = ["add", scopeAgentId, targetAgentId, alias.trim()].join(":");
    if (blocksAgentCommunicationIntent(mutationFailure, "add_contact", intentKey)) {
      return false;
    }
    setPendingAgentId(targetAgentId);
    clearMatchingMutationFailure(setMutationFailure, "add_contact", intentKey);
    try {
      const item = await upsertAgentContactApi(scopeAgentId, targetAgentId, alias);
      if (activeAgentIdRef.current === scopeAgentId) {
        setContacts((current) => [
          ...current.filter((contact) => contact.contact_agent_id !== targetAgentId),
          item,
        ]);
        setSelectedContactId(targetAgentId);
        clearMatchingMutationFailure(setMutationFailure, "add_contact", intentKey);
      }
      return true;
    } catch (mutationError) {
      if (activeAgentIdRef.current === scopeAgentId) {
        setMutationFailure(buildAgentCommunicationMutationFailure(
          mutationError,
          "add_contact",
          intentKey,
          "添加联系人没有完成",
          targetAgentId,
        ));
      }
      return false;
    } finally {
      if (activeAgentIdRef.current === scopeAgentId) {
        setPendingAgentId(null);
      }
    }
  }, [mutationFailure, pendingAgentId, scopeAgentId]);

  const createConversation = useCallback(async (title?: string): Promise<string | null> => {
    const roomId = roomContexts[0]?.room.id;
    if (!roomId) {
      return null;
    }
    const intentKey = ["conversation", roomId, title?.trim() ?? ""].join(":");
    if (blocksAgentCommunicationIntent(
      mutationFailure,
      "create_conversation",
      intentKey,
    )) {
      return null;
    }
    clearMatchingMutationFailure(setMutationFailure, "create_conversation", intentKey);
    try {
      const created = await createRoomConversation(roomId, { title });
      if (activeAgentIdRef.current !== scopeAgentId) {
        return null;
      }
      setRoomContexts((current) => [
        created,
        ...current.filter((item) => item.conversation.id !== created.conversation.id),
      ]);
      setConversationId(created.conversation.id);
      clearMatchingMutationFailure(setMutationFailure, "create_conversation", intentKey);
      return created.conversation.id;
    } catch (mutationError) {
      if (activeAgentIdRef.current === scopeAgentId) {
        setMutationFailure(buildAgentCommunicationMutationFailure(
          mutationError,
          "create_conversation",
          intentKey,
          "创建会话没有完成",
        ));
      }
      return null;
    }
  }, [mutationFailure, roomContexts, scopeAgentId]);

  const removeContact = useCallback(async (contactAgentId: string): Promise<boolean> => {
    const targetAgentId = contactAgentId.trim();
    if (!scopeAgentId || !targetAgentId || isRemoving) {
      return false;
    }
    const intentKey = ["remove", scopeAgentId, targetAgentId].join(":");
    if (blocksAgentCommunicationIntent(mutationFailure, "remove_contact", intentKey)) {
      return false;
    }
    setIsRemoving(true);
    clearMatchingMutationFailure(setMutationFailure, "remove_contact", intentKey);
    try {
      await deleteAgentContactApi(scopeAgentId, targetAgentId);
      if (activeAgentIdRef.current === scopeAgentId) {
        setContacts((current) => current.filter(
          (contact) => contact.contact_agent_id !== targetAgentId,
        ));
        setSelectedContactId((current) => current === targetAgentId ? null : current);
        setRoomContexts([]);
        setConversationId(null);
        setDirectEvents([]);
        clearMatchingMutationFailure(setMutationFailure, "remove_contact", intentKey);
      }
      return true;
    } catch (mutationError) {
      if (activeAgentIdRef.current === scopeAgentId) {
        setMutationFailure(buildAgentCommunicationMutationFailure(
          mutationError,
          "remove_contact",
          intentKey,
          "删除联系人没有完成",
          targetAgentId,
        ));
      }
      return false;
    } finally {
      if (activeAgentIdRef.current === scopeAgentId) {
        setIsRemoving(false);
      }
    }
  }, [isRemoving, mutationFailure, scopeAgentId]);

  const sendMessage = useCallback(async (content: string): Promise<void> => {
    const normalizedContent = content.trim();
    if (!scopeAgentId || !selectedContactId || !normalizedContent || isSending) {
      return;
    }
    const intentKey = [
      "send",
      scopeAgentId,
      selectedContactId,
      conversationId ?? "new",
      normalizedContent,
    ].join(":");
    if (blocksAgentCommunicationIntent(mutationFailure, "send_message", intentKey)) {
      throw requestAcceptanceUnknownError("消息结果仍在核对中");
    }
    setIsSending(true);
    clearMatchingMutationFailure(setMutationFailure, "send_message", intentKey);
    try {
      const result = await sendAgentCommunicationMessageApi(scopeAgentId, {
        content: normalizedContent,
        conversation_id: conversationId ?? undefined,
        target_id: selectedContactId,
          target_type: "agent",
      });
      if (activeAgentIdRef.current !== scopeAgentId) {
        return;
      }
      clearMatchingMutationFailure(setMutationFailure, "send_message", intentKey);
      if (conversationId) {
        await loadMessages(false);
      } else {
        setContacts((current) => current.map((contact) => (
          contact.contact_agent_id === selectedContactId
            ? { ...contact, direct_room_id: result.room_id }
            : contact
        )));
        setConversationId(result.conversation_id);
        void loadTarget();
      }
    } catch (mutationError) {
      const failure = buildAgentCommunicationMutationFailure(
        mutationError,
        "send_message",
        intentKey,
        "消息没有发送完成",
      );
      if (activeAgentIdRef.current === scopeAgentId) {
        setMutationFailure(failure);
      }
      throw failure.blocksRepeat
        ? requestAcceptanceUnknownError(failure.message)
        : new Error(failure.message);
    } finally {
      if (activeAgentIdRef.current === scopeAgentId) {
        setIsSending(false);
      }
    }
  }, [
    conversationId,
    isSending,
    loadMessages,
    loadTarget,
    mutationFailure,
    scopeAgentId,
    selectedContactId,
  ]);

  return {
    addContact,
    clearMutationFailure: () => setMutationFailure(null),
    clearSelection: () => setSelectedContactId(null),
    contacts,
    conversationId,
    conversationFailure,
    createConversation,
    directEvents,
    directoryFailure,
    hasMoreHistory,
    historyPrependToken,
    isDirectoryLoading,
    isHistoryLoading,
    isMessagesLoading,
    isSending,
    mutationFailure,
    pendingAgentId,
    loadOlderMessages,
    refresh: (kind) => {
      switch (kind) {
        case "directory":
          void loadDirectory();
          return;
        case "channel":
          void loadTarget();
          return;
        case "history":
          if (roomId && conversationId) {
            void loadOlderMessages();
          } else {
            void loadTarget();
          }
          return;
        case "messages":
          if (roomId && conversationId) {
            void loadMessages(true, true);
          } else {
            void loadTarget();
          }
          return;
        default:
          void loadDirectory();
          void loadTarget();
          void loadMessages(true, true);
      }
    },
    roomContexts,
    removeContact,
    selectedContactId,
    selectContact: setSelectedContactId,
    selectConversation: setConversationId,
    sendMessage,
  };
}

function requestAcceptanceUnknownError(message: string): Error {
  const error = new Error(message);
  error.name = "RequestAcceptanceUnknownError";
  return error;
}

function clearMatchingMutationFailure(
  setFailure: React.Dispatch<React.SetStateAction<AgentCommunicationMutationFailure | null>>,
  kind: AgentCommunicationMutationFailure["kind"],
  intentKey: string,
) {
  setFailure((current) => (
    current?.kind === kind && current.intentKey === intentKey ? null : current
  ));
}

function invalidatesReadSnapshot(error: unknown): boolean {
  return Boolean(getResourceFailure(error, "").access) || isNotFound(error);
}

function isNotFound(error: unknown): boolean {
  return error instanceof ApiRequestError && error.status === 404;
}

function mergePrivateEvents(
  earlier: AgentPrivateEvent[],
  later: AgentPrivateEvent[],
): AgentPrivateEvent[] {
  return Array.from(new Map(
    [...earlier, ...later].map((event) => [event.message_id, event]),
  ).values()).sort((left, right) => (
    left.timestamp - right.timestamp || left.message_id.localeCompare(right.message_id)
  ));
}
