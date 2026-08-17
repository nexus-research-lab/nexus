/**
 * INPUT: 当前详情 Agent id、好友私域记录 API 与通讯发送命令。
 * OUTPUT: Agent 视角的好友目录、Session、私信时间线和发送事务。
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
import { useAppEventSubscription, useWebSocket } from "@/lib/websocket";
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import type { AgentContact } from "@/types/agent/agent";
import type { AgentPrivateEvent } from "@/types/agent/private-domain";
import type { RoomContextAggregate } from "@/types/conversation/room";

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
  directEvents: AgentPrivateEvent[];
  error: string | null;
  hasMoreHistory: boolean;
  historyPrependToken: number;
  isDirectoryLoading: boolean;
  isHistoryLoading: boolean;
  isMessagesLoading: boolean;
  isSending: boolean;
  pendingAgentId: string | null;
  roomContexts: RoomContextAggregate[];
  selectedContactId: string | null;
  addContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  clearSelection: () => void;
  createConversation: (title?: string) => Promise<string | null>;
  loadOlderMessages: () => Promise<boolean>;
  removeContact: (contactAgentId: string) => Promise<boolean>;
  refresh: () => void;
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
  const [error, setError] = useState<string | null>(null);
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
  activeTargetKeyRef.current = targetKey;
  activeMessageKeyRef.current = messageKey;

  const loadDirectory = useCallback(async () => {
    if (!scopeAgentId) {
      return;
    }
    setIsDirectoryLoading(true);
    setError(null);
    try {
      const nextContacts = await listAgentContactsApi(scopeAgentId);
      if (activeAgentIdRef.current !== scopeAgentId) {
        return;
      }
      setContacts(nextContacts);
      setSelectedContactId((current) => (
        current && nextContacts.some((item) => item.contact_agent_id === current)
          ? current
          : nextContacts[0]?.contact_agent_id ?? null
      ));
    } catch (loadError) {
      if (activeAgentIdRef.current === scopeAgentId) {
        setError(errorMessage(loadError, "加载联系人失败"));
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
    setError(null);
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
    setError(null);
    try {
      const opened = await openAgentContactChannelApi(scopeAgentId, selectedContactId);
      const loadedContexts = await getRoomContexts(opened.room.id);
      const contexts = loadedContexts.length > 0 ? loadedContexts : [opened];
      if (activeTargetKeyRef.current !== requestKey) {
        return;
      }
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
        setRoomContexts([]);
        setConversationId(null);
        setDirectEvents([]);
        setError(loadError instanceof ApiRequestError && loadError.status === 404
          ? null
          : errorMessage(loadError, "打开联络会话失败"));
      }
    } finally {
      if (activeTargetKeyRef.current === requestKey) {
        setIsMessagesLoading(false);
      }
    }
  }, [scopeAgentId, selectedContactId, targetKey]);

  useEffect(() => {
    setRoomContexts([]);
    setConversationId(null);
    setDirectEvents([]);
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
        setError(null);
      }
    } catch (loadError) {
      if (activeMessageKeyRef.current === requestKey) {
        setError(errorMessage(loadError, "加载消息失败"));
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
      setHasMoreHistory(page.has_more);
      setHistoryCursor({
        beforeMessageId: page.next_before_message_id ?? null,
        beforeTimestamp: page.next_before_timestamp ?? null,
        threadId: historyCursor.threadId,
      });
      if (page.items.length === 0) {
        return false;
      }
      setDirectEvents((current) => mergePrivateEvents(page.items, current));
      setHistoryPrependToken((current) => current + 1);
      setError(null);
      return true;
    } catch (loadError) {
      if (activeMessageKeyRef.current === requestKey) {
        setError(errorMessage(loadError, "加载更早消息失败"));
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
    setPendingAgentId(targetAgentId);
    setError(null);
    try {
      const item = await upsertAgentContactApi(scopeAgentId, targetAgentId, alias);
      if (activeAgentIdRef.current === scopeAgentId) {
        setContacts((current) => [
          ...current.filter((contact) => contact.contact_agent_id !== targetAgentId),
          item,
        ]);
        setSelectedContactId(targetAgentId);
      }
      return true;
    } catch (mutationError) {
      setError(errorMessage(mutationError, "添加联系人失败"));
      return false;
    } finally {
      if (activeAgentIdRef.current === scopeAgentId) {
        setPendingAgentId(null);
      }
    }
  }, [pendingAgentId, scopeAgentId]);

  const createConversation = useCallback(async (title?: string): Promise<string | null> => {
    const roomId = roomContexts[0]?.room.id;
    if (!roomId) {
      return null;
    }
    try {
      const created = await createRoomConversation(roomId, { title });
      setRoomContexts((current) => [
        created,
        ...current.filter((item) => item.conversation.id !== created.conversation.id),
      ]);
      setConversationId(created.conversation.id);
      return created.conversation.id;
    } catch (mutationError) {
      setError(errorMessage(mutationError, "创建会话失败"));
      return null;
    }
  }, [roomContexts]);

  const removeContact = useCallback(async (contactAgentId: string): Promise<boolean> => {
    const targetAgentId = contactAgentId.trim();
    if (!scopeAgentId || !targetAgentId || isRemoving) {
      return false;
    }
    setIsRemoving(true);
    setError(null);
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
      }
      return true;
    } catch (mutationError) {
      setError(errorMessage(mutationError, "删除好友失败"));
      return false;
    } finally {
      if (activeAgentIdRef.current === scopeAgentId) {
        setIsRemoving(false);
      }
    }
  }, [isRemoving, scopeAgentId]);

  const sendMessage = useCallback(async (content: string): Promise<void> => {
    if (!scopeAgentId || !selectedContactId || isSending) {
      throw new Error("当前联络会话尚未就绪");
    }
    setIsSending(true);
    setError(null);
    try {
      const result = await sendAgentCommunicationMessageApi(scopeAgentId, {
        content,
        conversation_id: conversationId ?? undefined,
        target_id: selectedContactId,
        target_type: "agent",
      });
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
      setError(errorMessage(mutationError, "发送消息失败"));
      throw mutationError;
    } finally {
      setIsSending(false);
    }
  }, [conversationId, isSending, loadMessages, loadTarget, scopeAgentId, selectedContactId]);

  return {
    addContact,
    clearSelection: () => setSelectedContactId(null),
    contacts,
    conversationId,
    createConversation,
    directEvents,
    error,
    hasMoreHistory,
    historyPrependToken,
    isDirectoryLoading,
    isHistoryLoading,
    isMessagesLoading,
    isSending,
    pendingAgentId,
    loadOlderMessages,
    refresh: () => {
      void loadDirectory();
      void loadTarget();
      void loadMessages(true);
    },
    roomContexts,
    removeContact,
    selectedContactId,
    selectContact: setSelectedContactId,
    selectConversation: setConversationId,
    sendMessage,
  };
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
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
