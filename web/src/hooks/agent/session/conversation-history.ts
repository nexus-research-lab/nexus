/**
 * INPUT: 当前 Session 身份、round 游标、分页/around 请求与取消信号。
 * OUTPUT: 代次隔离、可取消且按完整 round 有界合并的历史窗口。
 * POS: Agent 会话增量历史请求与提交边界。
 */
import type {
  Dispatch,
  MutableRefObject,
  RefObject,
  SetStateAction,
} from "react";

import { getRoomConversationMessages } from "@/lib/api/conversation/room-resource-api";
import { getSessionMessagesApi } from "@/lib/api/conversation/session-api";
import type { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import type { ConversationMessagePage } from "@/types/conversation/history";
import type { Message } from "@/types/conversation/message/entity";

import {
  mergeLoadedMessages,
  sortMessages,
} from "../message/message-collection-model";
import {
  boundLoadedMessages,
  loadedMessageRoundIds,
} from "../message/message-window-model";
import {
  planOlderHistoryRequest,
  planRoundWindowHistoryRequest,
  type AgentConversationHistoryCursor,
  type ConversationHistoryRequest,
} from "./conversation-history-model";

interface AgentConversationHistoryContext {
  activeSessionKeyRef: RefObject<string | null>;
  historyCursorRef: MutableRefObject<AgentConversationHistoryCursor>;
  identity: AgentConversationIdentity | null;
  signal: AbortSignal;
  setError: Dispatch<SetStateAction<string | null>>;
  setHasMoreHistory: (nextValue: boolean) => void;
  setMessages: Dispatch<SetStateAction<Message[]>>;
}

interface LoadOlderAgentConversationMessagesParams
  extends AgentConversationHistoryContext {
  hasMoreHistoryRef: RefObject<boolean>;
  isHistoryLoadingRef: RefObject<boolean>;
  setHistoryLoading: (nextValue: boolean) => void;
  setHistoryPrependToken: Dispatch<SetStateAction<number>>;
}

interface LoadRoundWindowMessagesParams
  extends AgentConversationHistoryContext {
  isRoundWindowLoadingRef: MutableRefObject<boolean>;
  onRoundResolved: (roundId: string) => void;
  roundId: string;
}

async function requestHistoryPage(
  request: ConversationHistoryRequest,
  isCurrent: () => boolean,
  signal: AbortSignal,
): Promise<ConversationMessagePage | null> {
  for (;;) {
    const page = request.source.kind === "room"
      ? await getRoomConversationMessages(
        request.source.roomId,
        request.source.conversationId,
        request.query,
        signal,
      )
      : await getSessionMessagesApi(
        request.source.sessionKey,
        request.query,
        signal,
      );
    if (!page.indexing) {
      return page;
    }
    if (!isCurrent()) {
      return null;
    }
    await waitForHistoryIndex(page.retry_after_ms, signal);
    if (!isCurrent()) {
      return null;
    }
  }
}

function waitForHistoryIndex(
  retryAfterMs: number,
  signal: AbortSignal,
): Promise<void> {
  if (signal.aborted) {
    return Promise.reject(new DOMException("Aborted", "AbortError"));
  }
  const delay = Math.min(Math.max(retryAfterMs, 100), 5_000);
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      signal.removeEventListener("abort", handleAbort);
      resolve();
    }, delay);
    const handleAbort = (): void => {
      window.clearTimeout(timeout);
      reject(new DOMException("Aborted", "AbortError"));
    };
    signal.addEventListener("abort", handleAbort, { once: true });
  });
}

function isCurrentHistoryRequest(
  request: ConversationHistoryRequest,
  activeSessionKeyRef: RefObject<string | null>,
): boolean {
  return activeSessionKeyRef.current === request.activeSessionKey;
}

function updateHistoryCursor(
  cursorRef: MutableRefObject<AgentConversationHistoryCursor>,
  page: ConversationMessagePage,
): void {
  cursorRef.current = {
    before_round_id: page.next_before_round_id,
    before_round_timestamp: page.next_before_round_timestamp,
  };
}

function commitOlderHistoryPage(
  page: ConversationMessagePage,
  context: LoadOlderAgentConversationMessagesParams,
): boolean {
  const sortedMessages = sortMessages(page.items);
  if (sortedMessages.length === 0) {
    context.historyCursorRef.current = {
      before_round_id: null,
      before_round_timestamp: null,
    };
    context.setHasMoreHistory(false);
    return false;
  }

  context.setMessages((currentMessages) =>
    boundLoadedMessages(
      mergeLoadedMessages(sortedMessages, currentMessages),
      {
        anchorRoundIds: loadedMessageRoundIds(sortedMessages),
        preference: "anchor",
      },
    ),
  );
  updateHistoryCursor(context.historyCursorRef, page);
  context.setHasMoreHistory(page.has_more);
  context.setHistoryPrependToken((currentToken) => currentToken + 1);
  return true;
}

function commitRoundWindowHistoryPage(
  page: ConversationMessagePage,
  context: LoadRoundWindowMessagesParams,
): boolean {
  const sortedMessages = sortMessages(page.items);
  if (sortedMessages.length === 0) {
    return false;
  }

  context.setMessages((currentMessages) =>
    boundLoadedMessages(
      mergeLoadedMessages(sortedMessages, currentMessages),
      {
        anchorRoundIds: loadedMessageRoundIds(sortedMessages),
        preference: "anchor",
      },
    ),
  );
  if (page.next_before_round_timestamp) {
    updateHistoryCursor(context.historyCursorRef, page);
    context.setHasMoreHistory(page.has_more);
  }
  return true;
}

function reportHistoryLoadError(
  error: unknown,
  context: AgentConversationHistoryContext,
  logMessage: string,
  fallbackMessage: string,
): void {
  console.error(logMessage, error);
  context.setError(error instanceof Error ? error.message : fallbackMessage);
}

export async function loadOlderAgentConversationMessages(
  context: LoadOlderAgentConversationMessagesParams,
): Promise<boolean> {
  const request = planOlderHistoryRequest({
    activeSessionKey: context.activeSessionKeyRef.current,
    cursor: context.historyCursorRef.current,
    hasMore: context.hasMoreHistoryRef.current,
    identity: context.identity,
    isLoading: context.isHistoryLoadingRef.current,
  });
  if (!request) {
    return false;
  }

  context.setHistoryLoading(true);
  try {
    const page = await requestHistoryPage(
      request,
      () => isCurrentHistoryRequest(request, context.activeSessionKeyRef),
      context.signal,
    );
    return page && isCurrentHistoryRequest(request, context.activeSessionKeyRef)
      ? commitOlderHistoryPage(page, context)
      : false;
  } catch (error) {
    if (context.signal.aborted) {
      return false;
    }
    if (!isCurrentHistoryRequest(request, context.activeSessionKeyRef)) {
      return false;
    }
    reportHistoryLoadError(
      error,
      context,
      "[useAgentConversation] 加载更早消息失败:",
      "Failed to load older messages",
    );
    return false;
  } finally {
    if (isCurrentHistoryRequest(request, context.activeSessionKeyRef)) {
      context.setHistoryLoading(false);
    }
  }
}

export async function loadAgentConversationMessagesAroundRound(
  context: LoadRoundWindowMessagesParams,
): Promise<boolean> {
  const request = planRoundWindowHistoryRequest({
    activeSessionKey: context.activeSessionKeyRef.current,
    identity: context.identity,
    isLoading: context.isRoundWindowLoadingRef.current,
    roundId: context.roundId,
  });
  if (!request) {
    return false;
  }

  context.isRoundWindowLoadingRef.current = true;
  try {
    const page = await requestHistoryPage(
      request,
      () => isCurrentHistoryRequest(request, context.activeSessionKeyRef),
      context.signal,
    );
    if (!page || !isCurrentHistoryRequest(request, context.activeSessionKeyRef)) {
      return false;
    }
    context.onRoundResolved(context.roundId);
    return commitRoundWindowHistoryPage(page, context);
  } catch (error) {
    if (context.signal.aborted) {
      return false;
    }
    if (!isCurrentHistoryRequest(request, context.activeSessionKeyRef)) {
      return false;
    }
    reportHistoryLoadError(
      error,
      context,
      "[useAgentConversation] 加载目标轮次附近消息失败:",
      "Failed to load target messages",
    );
    return false;
  } finally {
    context.isRoundWindowLoadingRef.current = false;
  }
}
