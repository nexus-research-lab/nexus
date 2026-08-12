/**
 * INPUT: 当前详情 Agent id、好友私域记录 API 与通讯发送命令。
 * OUTPUT: Agent 视角的好友目录、Session、私信时间线和发送事务。
 * POS: Contacts 页面好友私聊客户端的数据与命令边界。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  openAgentContactChannelApi,
  sendAgentCommunicationMessageApi,
} from "@/lib/api/agent/agent-communication-api";
import {
  listAgentContactsApi,
  upsertAgentContactApi,
} from "@/lib/api/agent/agent-api";
import {
  listAgentPrivateEventsApi,
  listAgentPrivateThreadsApi,
} from "@/lib/api/agent/private-domain-api";
import { createRoomConversation } from "@/lib/api/conversation/room-command-api";
import { getRoomContexts } from "@/lib/api/conversation/room-resource-api";
import type { AgentContact } from "@/types/agent/agent";
import type { AgentPrivateEvent } from "@/types/agent/private-domain";
import type { RoomContextAggregate } from "@/types/conversation/room";

const MESSAGE_LIMIT = 160;
const MESSAGE_POLL_INTERVAL_MS = 3_000;

export interface AgentCommunicationResource {
  contacts: AgentContact[];
  conversationId: string | null;
  directEvents: AgentPrivateEvent[];
  error: string | null;
  isDirectoryLoading: boolean;
  isMessagesLoading: boolean;
  isSending: boolean;
  pendingAgentId: string | null;
  roomContexts: RoomContextAggregate[];
  selectedContactId: string | null;
  addContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  clearSelection: () => void;
  createConversation: (title?: string) => Promise<string | null>;
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
  const [isSending, setIsSending] = useState(false);
  const [pendingAgentId, setPendingAgentId] = useState<string | null>(null);
  const targetKey = `${scopeAgentId}:${selectedContactId ?? "none"}`;
  const messageKey = `${targetKey}:${conversationId ?? "none"}`;
  const activeTargetKeyRef = useRef(targetKey);
  const activeMessageKeyRef = useRef(messageKey);
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
      const opened = await openAgentContactChannelApi(
        scopeAgentId,
        selectedContactId,
      );
      const loadedContexts = await getRoomContexts(opened.room.id);
      const contexts = loadedContexts.length > 0 ? loadedContexts : [opened];
      if (activeTargetKeyRef.current !== requestKey) {
        return;
      }
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
        setError(errorMessage(loadError, "打开联络会话失败"));
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

  const loadMessages = useCallback(async (showLoading: boolean) => {
    const requestKey = messageKey;
    const roomId = roomContexts[0]?.room.id ?? null;
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
      const events = thread
        ? (await listAgentPrivateEventsApi(
          scopeAgentId,
          thread.thread_id,
          query,
        )).items
        : [];
      if (activeMessageKeyRef.current === requestKey) {
        setDirectEvents(events);
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
  }, [conversationId, messageKey, roomContexts, scopeAgentId, selectedContactId]);

  useEffect(() => {
    if (!conversationId) {
      return undefined;
    }
    void loadMessages(true);
    const timer = window.setInterval(() => {
      void loadMessages(false);
    }, MESSAGE_POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [conversationId, loadMessages]);

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
    isDirectoryLoading,
    isMessagesLoading,
    isSending,
    pendingAgentId,
    refresh: () => {
      void loadDirectory();
      void loadTarget();
      void loadMessages(true);
    },
    roomContexts,
    selectedContactId,
    selectContact: setSelectedContactId,
    selectConversation: setConversationId,
    sendMessage,
  };
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}
