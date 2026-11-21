/**
 * INPUT: 当前 Session 身份、驻留消息窗口、round 游标、分页/around 请求与取消信号。
 * OUTPUT: 代次隔离、可取消的连续历史窗口：向前/向后扩展一页、围绕目标重建或回到最新。
 * POS: Agent 会话增量历史请求与提交边界。
 */
import type {
  MutableRefObject,
  RefObject,
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
  boundMessageWindow,
  isPinnedWindowMessage,
  loadedMessageRoundIds,
  messageWindowKey,
  type LoadedMessageWindow,
} from "../message/message-window-model";
import type { MessageWindowUpdater } from "../message/use-agent-message-collection";
import {
  planLatestHistoryRequest,
  planNewerHistoryRequest,
  planOlderHistoryRequest,
  planRoundWindowHistoryRequest,
  type AgentConversationHistoryCursor,
  type ConversationHistoryRequest,
} from "./conversation-history-model";
import { requestConversationHistoryPageUntilReady } from "./conversation-history-request";

interface AgentConversationHistoryContext {
  activeSessionKeyRef: RefObject<string | null>;
  commitHistoryCursor: (
    cursor: AgentConversationHistoryCursor,
    hasMore: boolean,
  ) => void;
  historyCursorRef: RefObject<AgentConversationHistoryCursor>;
  identity: AgentConversationIdentity | null;
  signal: AbortSignal;
  updateMessageWindow: (updater: MessageWindowUpdater) => void;
  windowRef: RefObject<LoadedMessageWindow>;
}

interface LoadOlderAgentConversationMessagesParams
  extends AgentConversationHistoryContext {
  hasMoreHistoryRef: RefObject<boolean>;
  isHistoryLoadingRef: RefObject<boolean>;
  setHistoryLoading: (nextValue: boolean) => void;
}

interface LoadRoundWindowMessagesParams
  extends AgentConversationHistoryContext {
  isRoundWindowLoadingRef: MutableRefObject<boolean>;
  onRoundResolved: (roundId: string) => void;
  roundId: string;
}

interface LoadWindowEdgeMessagesParams
  extends AgentConversationHistoryContext {
  isRoundWindowLoadingRef: MutableRefObject<boolean>;
}

async function requestHistoryPage(
  request: ConversationHistoryRequest,
  signal: AbortSignal,
): Promise<ConversationMessagePage> {
  return requestConversationHistoryPageUntilReady({
    loadPage: () => request.source.kind === "room"
      ? getRoomConversationMessages(
        request.source.roomId,
        request.source.conversationId,
        request.query,
        signal,
      )
      : getSessionMessagesApi(
        request.source.sessionKey,
        request.query,
        signal,
      ),
    signal,
  });
}

function isCurrentHistoryRequest(
  request: ConversationHistoryRequest,
  activeSessionKeyRef: RefObject<string | null>,
): boolean {
  return activeSessionKeyRef.current === request.activeSessionKey;
}

function commitPageCursor(
  context: AgentConversationHistoryContext,
  page: ConversationMessagePage,
  pageMessages: Message[],
  hasMore: boolean,
): void {
  const reachedStart = !page.has_more || page.next_before_round_id === null;
  context.commitHistoryCursor({
    before_round_id: page.next_before_round_id,
    before_round_timestamp: page.next_before_round_timestamp,
    // 会话起点不会改变：没有到达起点的页保留此前已确认的起点。
    start_round_id: reachedStart
      ? loadedMessageRoundIds(pageMessages)[0] ?? null
      : context.historyCursorRef.current.start_round_id ?? null,
  }, hasMore);
}

function commitOlderHistoryPage(
  page: ConversationMessagePage,
  context: LoadOlderAgentConversationMessagesParams,
): boolean {
  const sortedMessages = sortMessages(page.items);
  if (sortedMessages.length === 0) {
    // 空页说明请求游标之前已无历史：把当前窗口起点记为会话起点。
    context.commitHistoryCursor({
      before_round_id: null,
      before_round_timestamp: null,
      start_round_id:
        loadedMessageRoundIds(context.windowRef.current.messages)[0] ?? null,
    }, false);
    return false;
  }

  // 向前扩展时读者位于窗口起点一侧，超出预算从最新一端让出。
  context.updateMessageWindow((current) => boundMessageWindow(
    {
      gapAfterRoundId: current.gapAfterRoundId,
      messages: mergeLoadedMessages(sortedMessages, current.messages),
    },
    { evict: "tail" },
  ));
  commitPageCursor(context, page, sortedMessages, page.has_more);
  return true;
}

/**
 * 导航目标不在驻留窗口内时围绕目标重建窗口：与当前窗口有交集的页直接并入，
 * 否则丢弃旧窗口，只保留未落库/流式中的实时尾部，并在新窗口之后记下断点。
 */
function commitRoundWindowHistoryPage(
  page: ConversationMessagePage,
  context: LoadRoundWindowMessagesParams,
): boolean {
  const sortedMessages = sortMessages(page.items);
  if (sortedMessages.length === 0) {
    return false;
  }

  const pageRoundIds = loadedMessageRoundIds(sortedMessages);
  context.updateMessageWindow((current) => {
    const pageRoundIdSet = new Set(pageRoundIds);
    const overlaps = current.messages.some(
      (message) => pageRoundIdSet.has(messageWindowKey(message)),
    );
    if (overlaps) {
      return boundMessageWindow(
        {
          gapAfterRoundId: current.gapAfterRoundId,
          messages: mergeLoadedMessages(sortedMessages, current.messages),
        },
        { evict: resolveFarEdge(current.messages, pageRoundIdSet) },
      );
    }
    const liveTail = current.messages.filter(isPinnedWindowMessage);
    return {
      gapAfterRoundId: pageRoundIds.at(-1) ?? null,
      messages: mergeLoadedMessages(sortedMessages, liveTail),
    };
  });
  commitPageCursor(context, page, sortedMessages, page.next_before_round_id !== null);
  return true;
}

/** 目标页落在窗口前半段时让出最新一端，否则让出最旧一端。 */
function resolveFarEdge(
  messages: Message[],
  pageRoundIds: Set<string>,
): "head" | "tail" {
  const roundIds = loadedMessageRoundIds(messages);
  const firstIndex = roundIds.findIndex((roundId) => pageRoundIds.has(roundId));
  return firstIndex >= 0 && firstIndex < roundIds.length / 2 ? "tail" : "head";
}

/**
 * 向后扩展以断点 round 为中心取一页：服务端按 transcript 顺序返回，
 * 因此新页必然与窗口相接。页内出现断点之后已驻留的 round，或没有带来任何
 * 新 round，都说明窗口已经连到实时尾部。
 */
function commitNewerHistoryPage(
  page: ConversationMessagePage,
  gapRoundId: string,
  context: LoadWindowEdgeMessagesParams,
): boolean {
  const sortedMessages = sortMessages(page.items);
  if (sortedMessages.length === 0) {
    return false;
  }

  const pageRoundIds = loadedMessageRoundIds(sortedMessages);
  context.updateMessageWindow((current) => {
    if (current.gapAfterRoundId !== gapRoundId) {
      return current;
    }
    const residentRoundIds = new Set(loadedMessageRoundIds(current.messages));
    const newerRoundIds = pageRoundIds.slice(pageRoundIds.indexOf(gapRoundId) + 1);
    const reachedTail = newerRoundIds.length === 0
      || newerRoundIds.some((roundId) => residentRoundIds.has(roundId));
    return boundMessageWindow(
      {
        gapAfterRoundId: reachedTail ? null : newerRoundIds.at(-1) ?? null,
        messages: mergeLoadedMessages(sortedMessages, current.messages),
      },
      { evict: "head" },
    );
  });
  return true;
}

/** 回到最新：丢弃断点及其之前的旧窗口，保留实时尾部并接上最新一页。 */
function commitLatestHistoryPage(
  page: ConversationMessagePage,
  context: LoadWindowEdgeMessagesParams,
): boolean {
  const sortedMessages = sortMessages(page.items);
  context.updateMessageWindow((current) => {
    const gapRoundId = current.gapAfterRoundId;
    if (gapRoundId === null) {
      return current;
    }
    const roundIds = loadedMessageRoundIds(current.messages);
    const detached = new Set(roundIds.slice(0, roundIds.indexOf(gapRoundId) + 1));
    const tail = current.messages.filter((message) => (
      isPinnedWindowMessage(message)
      || !detached.has(messageWindowKey(message))
    ));
    return {
      gapAfterRoundId: null,
      messages: mergeLoadedMessages(sortedMessages, tail),
    };
  });
  commitPageCursor(context, page, sortedMessages, page.has_more);
  return true;
}

function reportHistoryLoadError(
  error: unknown,
  logMessage: string,
): void {
  console.error(logMessage, error);
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
    residentRoundIds: loadedMessageRoundIds(
      context.windowRef.current.messages,
    ),
  });
  if (!request) {
    return false;
  }

  context.setHistoryLoading(true);
  try {
    const page = await requestHistoryPage(
      request,
      context.signal,
    );
    return isCurrentHistoryRequest(request, context.activeSessionKeyRef)
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
      "[useAgentConversation] 加载更早消息失败:",
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
      context.signal,
    );
    if (!isCurrentHistoryRequest(request, context.activeSessionKeyRef)) {
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
      "[useAgentConversation] 加载目标轮次附近消息失败:",
    );
    return false;
  } finally {
    context.isRoundWindowLoadingRef.current = false;
  }
}

/** 把窗口向更新的一侧扩展一页；窗口已连到最新时不发请求。 */
export async function loadNewerAgentConversationMessages(
  context: LoadWindowEdgeMessagesParams,
): Promise<boolean> {
  const gapRoundId = context.windowRef.current.gapAfterRoundId;
  const request = planNewerHistoryRequest({
    activeSessionKey: context.activeSessionKeyRef.current,
    gapRoundId,
    identity: context.identity,
    isLoading: context.isRoundWindowLoadingRef.current,
  });
  if (!request || gapRoundId === null) {
    return false;
  }
  return runWindowEdgeRequest(
    request,
    context,
    (page) => commitNewerHistoryPage(page, gapRoundId, context),
    "[useAgentConversation] 加载更新消息失败:",
  );
}

/** 读者回到底部时丢弃脱离最新的旧窗口，重新接上最新一页。 */
export async function returnToLatestAgentConversationMessages(
  context: LoadWindowEdgeMessagesParams,
): Promise<boolean> {
  const request = planLatestHistoryRequest({
    activeSessionKey: context.activeSessionKeyRef.current,
    gapRoundId: context.windowRef.current.gapAfterRoundId,
    identity: context.identity,
    isLoading: context.isRoundWindowLoadingRef.current,
  });
  if (!request) {
    return false;
  }
  return runWindowEdgeRequest(
    request,
    context,
    (page) => commitLatestHistoryPage(page, context),
    "[useAgentConversation] 回到最新消息失败:",
  );
}

async function runWindowEdgeRequest(
  request: ConversationHistoryRequest,
  context: LoadWindowEdgeMessagesParams,
  commit: (page: ConversationMessagePage) => boolean,
  logMessage: string,
): Promise<boolean> {
  context.isRoundWindowLoadingRef.current = true;
  try {
    const page = await requestHistoryPage(request, context.signal);
    return isCurrentHistoryRequest(request, context.activeSessionKeyRef)
      ? commit(page)
      : false;
  } catch (error) {
    if (
      !context.signal.aborted
      && isCurrentHistoryRequest(request, context.activeSessionKeyRef)
    ) {
      reportHistoryLoadError(error, logMessage);
    }
    return false;
  } finally {
    context.isRoundWindowLoadingRef.current = false;
  }
}
