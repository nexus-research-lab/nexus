/**
 * INPUT: Session 身份、初始/重载请求和页面/易失快照状态。
 * OUTPUT: 可取消的首屏加载、代次栅栏与受限消息窗口提交。
 * POS: Agent Session 切换、创建、加载与清理状态机。
 */
import type { Dispatch, RefObject, SetStateAction } from "react";

import { getMessageHistoryRoundPageSize } from "@/config/conversation-policy";
import { getSessionMessagesApi } from "@/lib/api/conversation/session-api";
import { getRoomConversationMessages } from "@/lib/api/conversation/room-resource-api";
import {
  buildRoomSharedSessionKey,
  buildSessionKey,
} from "@/lib/conversation/session-key";
import { generateUuid } from "@/lib/uuid";
import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  AgentConversationIdentity,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import type { ConversationMessagePage } from "@/types/conversation/history";

import {
  mergeLoadedMessages,
  sortMessages,
} from "../message/message-collection-model";
import { latestAssistantResultErrorMessage } from "../message/assistant-message-model";
import { boundLoadedMessages } from "../message/message-window-model";

interface AgentConversationLifecycleRefs {
  activeSessionKey: RefObject<string | null>;
  backgroundMessages: RefObject<Map<string, Message[]>>;
  loadAbortController: RefObject<AbortController | null>;
  loadRequestId: RefObject<number>;
}

interface AgentConversationLifecycleState {
  setError: Dispatch<SetStateAction<string | null>>;
  setInputQueueItems: Dispatch<SetStateAction<InputQueueItem[]>>;
  setIsSessionLoading: Dispatch<SetStateAction<boolean>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
  setSessionKey: Dispatch<SetStateAction<string | null>>;
}

export interface AgentConversationLifecycleContext {
  identity: AgentConversationIdentity | null;
  refs: AgentConversationLifecycleRefs;
  state: AgentConversationLifecycleState;
  restoreVolatileSessionSnapshot: (sessionKey: string) => boolean;
  onSessionMessagesLoaded: (
    messages: Message[],
    meta: {
      hasMoreHistory: boolean;
      isReload: boolean;
      nextBeforeRoundId: string | null;
      nextBeforeRoundTimestamp: number | null;
      sessionKey: string;
    },
  ) => void;
}

function resetSessionView(
  context: AgentConversationLifecycleContext,
  nextError: string | null = null,
): void {
  const { state } = context;
  state.setMessages([]);
  state.setPendingAgentSlots([]);
  state.setInputQueueItems([]);
  state.setPendingPermissions([]);
  state.setError(nextError);
}

function createSessionKey(
  identity: AgentConversationIdentity | null,
): string {
  if (identity?.chat_type === "group" && identity.conversation_id) {
    return buildRoomSharedSessionKey(identity.conversation_id);
  }
  return buildSessionKey({
    channel: "ws",
    chat_type: "dm",
    ref: generateUuid(),
    agent_id: identity?.agent_id,
  });
}

export function startAgentSession(
  context: AgentConversationLifecycleContext,
): void {
  context.refs.loadAbortController.current?.abort();
  context.refs.loadAbortController.current = null;
  const sessionKey = createSessionKey(context.identity);
  context.refs.loadRequestId.current += 1;
  context.refs.activeSessionKey.current = sessionKey;
  context.state.setSessionKey(sessionKey);
  context.state.setIsSessionLoading(false);
  resetSessionView(context);
}

function isCurrentLoad(
  context: AgentConversationLifecycleContext,
  requestId: number,
  sessionKey: string,
): boolean {
  return (
    context.refs.loadRequestId.current === requestId &&
    context.refs.activeSessionKey.current === sessionKey
  );
}

function prepareSessionLoad(
  sessionKey: string,
  context: AgentConversationLifecycleContext,
  isReload: boolean,
): { requestId: number; controller: AbortController } {
  context.refs.loadAbortController.current?.abort();
  const controller = new AbortController();
  context.refs.loadAbortController.current = controller;
  const requestId = context.refs.loadRequestId.current + 1;
  context.refs.loadRequestId.current = requestId;
  context.refs.activeSessionKey.current = sessionKey;
  context.state.setSessionKey(sessionKey);

  if (isReload) {
    context.state.setError(null);
    return { controller, requestId };
  }

  context.state.setIsSessionLoading(true);
  const cachedMessages = context.refs.backgroundMessages.current.get(sessionKey);
  if (cachedMessages?.length) {
    context.state.setMessages(sortMessages(cachedMessages));
    context.state.setPendingPermissions([]);
    context.state.setError(null);
  } else {
    resetSessionView(context);
  }
  context.restoreVolatileSessionSnapshot(sessionKey);
  return { controller, requestId };
}

/**
 * 只读指定 Session 的 durable 消息，不改变当前页面 identity 或消息投影。
 * 请求级 ACK 恢复用它核对原 Session，避免用户已切到新会话时误读新页面。
 */
export async function readAgentSessionMessagePage(
  identity: AgentConversationIdentity | null,
  sessionKey: string,
  signal?: AbortSignal,
): Promise<ConversationMessagePage> {
  const query = { limit: getMessageHistoryRoundPageSize() };
  if (identity?.room_id && identity.conversation_id) {
    return getRoomConversationMessages(
      identity.room_id,
      identity.conversation_id,
      query,
      signal,
    );
  }
  return getSessionMessagesApi(sessionKey, query, signal);
}

/** ACK recovery 只读取捕获的原 Session；调用者可选地持有取消所有权。 */
export async function readAgentSessionMessages(
  identity: AgentConversationIdentity | null,
  sessionKey: string,
  signal?: AbortSignal,
): Promise<Message[]> {
  return (await readAgentSessionMessagePage(identity, sessionKey, signal)).items;
}

function commitSessionMessages(
  sessionKey: string,
  page: ConversationMessagePage,
  context: AgentConversationLifecycleContext,
  isReload: boolean,
): Message[] {
  const sortedMessages = sortMessages(page.items);
  let mergedMessages = sortedMessages;
  context.state.setMessages((currentMessages) => {
    mergedMessages = boundLoadedMessages(
      mergeLoadedMessages(sortedMessages, currentMessages),
      { preference: "latest" },
    );
    return mergedMessages;
  });
  const resultError = latestAssistantResultErrorMessage(mergedMessages);
  if (resultError) {
    context.state.setError(resultError);
  }
  context.onSessionMessagesLoaded(mergedMessages, {
    hasMoreHistory: page.has_more,
    isReload,
    nextBeforeRoundId: page.next_before_round_id,
    nextBeforeRoundTimestamp: page.next_before_round_timestamp,
    sessionKey,
  });
  context.refs.backgroundMessages.current.delete(sessionKey);
  return sortedMessages;
}

/**
 * 同会话重拉只刷新持久消息，运行态继续由 WebSocket 权威事件维护。
 * 请求编号同时隔离快速切换产生的过期响应。
 */
export async function loadAgentSession(
  sessionKey: string,
  context: AgentConversationLifecycleContext,
  isReload = false,
): Promise<Message[] | null> {
  const { controller, requestId } = prepareSessionLoad(
    sessionKey,
    context,
    isReload,
  );
  try {
    const page = await readAgentSessionMessagePage(
      context.identity,
      sessionKey,
      controller.signal,
    );
    if (isCurrentLoad(context, requestId, sessionKey)) {
      return commitSessionMessages(sessionKey, page, context, isReload);
    }
  } catch (error) {
    if (controller.signal.aborted) {
      return null;
    }
    if (isCurrentLoad(context, requestId, sessionKey)) {
      console.error("[loadSession] 加载 session 失败:", error);
      context.state.setError(
        error instanceof Error ? error.message : "Failed to load session",
      );
    }
  } finally {
    if (context.refs.loadAbortController.current === controller) {
      context.refs.loadAbortController.current = null;
    }
    if (!isReload && isCurrentLoad(context, requestId, sessionKey)) {
      context.state.setIsSessionLoading(false);
    }
  }
  return null;
}

export function clearAgentSession(
  context: AgentConversationLifecycleContext,
): void {
  context.refs.loadAbortController.current?.abort();
  context.refs.loadAbortController.current = null;
  context.refs.loadRequestId.current += 1;
  context.refs.activeSessionKey.current = null;
  context.state.setSessionKey(null);
  context.state.setIsSessionLoading(false);
  resetSessionView(context);
}

export function resetAgentSession(
  context: AgentConversationLifecycleContext,
): void {
  startAgentSession(context);
}
