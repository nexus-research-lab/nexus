import { Dispatch, MutableRefObject, RefObject, SetStateAction } from "react";
import { getMessageHistoryRoundPageSize } from "@/config/conversation-policy";
import { getSessionMessagesApi } from "@/lib/api/conversation/session-api";
import { getRoomConversationMessages } from "@/lib/api/conversation/room-resource-api";
import type { Message } from "@/types/conversation/message/entity";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import {
  mergeLoadedMessages,
  sortMessages,
} from "../message/message-collection-model";

const TARGET_ROUND_WINDOW_RADIUS = 1;

export interface AgentConversationHistoryCursor {
  before_round_id: string | null;
  before_round_timestamp: number | null;
}

export interface LoadOlderAgentConversationMessagesParams {
  active_session_key_ref: RefObject<string | null>;
  identity: AgentConversationIdentity | null;
  history_cursor_ref: MutableRefObject<AgentConversationHistoryCursor>;
  has_more_history_ref: RefObject<boolean>;
  is_history_loading_ref: RefObject<boolean>;
  set_history_loading: (nextValue: boolean) => void;
  set_has_more_history: (nextValue: boolean) => void;
  set_history_prepend_token: Dispatch<SetStateAction<number>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_error: Dispatch<SetStateAction<string | null>>;
}

export async function loadOlderAgentConversationMessages({
  active_session_key_ref: activeSessionKeyRef,
  identity,
  history_cursor_ref: historyCursorRef,
  has_more_history_ref: hasMoreHistoryRef,
  is_history_loading_ref: isHistoryLoadingRef,
  set_history_loading: setHistoryLoading,
  set_has_more_history: setHasMoreHistory,
  set_history_prepend_token: setHistoryPrependToken,
  set_messages: setMessages,
  set_error: setError,
}: LoadOlderAgentConversationMessagesParams): Promise<boolean> {
  const activeSessionKey = activeSessionKeyRef.current;
  const currentRoomId = identity?.room_id?.trim() ?? "";
  const currentConversationId = identity?.conversation_id?.trim() ?? "";
  const beforeRoundId = historyCursorRef.current.before_round_id;
  const beforeRoundTimestamp =
    historyCursorRef.current.before_round_timestamp;

  if (
    !activeSessionKey ||
    !hasMoreHistoryRef.current ||
    isHistoryLoadingRef.current ||
    !beforeRoundTimestamp
  ) {
    return false;
  }

  setHistoryLoading(true);
  try {
    const page = currentRoomId && currentConversationId
      ? await getRoomConversationMessages(
          currentRoomId,
          currentConversationId,
          {
            limit: getMessageHistoryRoundPageSize(),
            before_round_id: beforeRoundId,
            before_round_timestamp: beforeRoundTimestamp,
          },
        )
      : await getSessionMessagesApi(activeSessionKey, {
          limit: getMessageHistoryRoundPageSize(),
          before_round_id: beforeRoundId,
          before_round_timestamp: beforeRoundTimestamp,
        });
    if (activeSessionKeyRef.current !== activeSessionKey) {
      return false;
    }

    const sortedMessages = sortMessages(page.items ?? []);
    if (sortedMessages.length === 0) {
      historyCursorRef.current = {
        before_round_id: null,
        before_round_timestamp: null,
      };
      setHasMoreHistory(false);
      return false;
    }

    setMessages((currentMessages) =>
      mergeLoadedMessages(sortedMessages, currentMessages),
    );
    historyCursorRef.current = {
      before_round_id: page.next_before_round_id ?? null,
      before_round_timestamp: page.next_before_round_timestamp ?? null,
    };
    setHasMoreHistory(page.has_more ?? false);
    setHistoryPrependToken((currentToken) => currentToken + 1);
    return true;
  } catch (err) {
    if (activeSessionKeyRef.current !== activeSessionKey) {
      return false;
    }
    console.error("[useAgentConversation] 加载更早消息失败:", err);
    setError(
      err instanceof Error ? err.message : "Failed to load older messages",
    );
    return false;
  } finally {
    if (activeSessionKeyRef.current === activeSessionKey) {
      setHistoryLoading(false);
    }
  }
}

export async function loadAgentConversationMessagesAroundRound({
  active_session_key_ref: activeSessionKeyRef,
  identity,
  history_cursor_ref: historyCursorRef,
  is_round_window_loading_ref: isRoundWindowLoadingRef,
  round_id: roundId,
  set_has_more_history: setHasMoreHistory,
  set_messages: setMessages,
  set_error: setError,
}: Omit<
  LoadOlderAgentConversationMessagesParams,
  | "has_more_history_ref"
  | "is_history_loading_ref"
  | "set_history_loading"
  | "set_history_prepend_token"
> & {
  is_round_window_loading_ref: MutableRefObject<boolean>;
  round_id: string;
}): Promise<boolean> {
  const activeSessionKey = activeSessionKeyRef.current;
  const currentRoomId = identity?.room_id?.trim() ?? "";
  const currentConversationId = identity?.conversation_id?.trim() ?? "";
  const targetRoundId = roundId.trim();

  if (!activeSessionKey || !targetRoundId || isRoundWindowLoadingRef.current) {
    return false;
  }

  isRoundWindowLoadingRef.current = true;
  try {
    const page = currentRoomId && currentConversationId
      ? await getRoomConversationMessages(
          currentRoomId,
          currentConversationId,
          {
            around_round_id: targetRoundId,
            around_limit: TARGET_ROUND_WINDOW_RADIUS,
          },
        )
      : await getSessionMessagesApi(activeSessionKey, {
          around_round_id: targetRoundId,
          around_limit: TARGET_ROUND_WINDOW_RADIUS,
        });
    if (activeSessionKeyRef.current !== activeSessionKey) {
      return false;
    }

    const sortedMessages = sortMessages(page.items ?? []);
    if (sortedMessages.length === 0) {
      return false;
    }

    setMessages((currentMessages) =>
      mergeLoadedMessages(sortedMessages, currentMessages),
    );
    if (page.next_before_round_timestamp) {
      historyCursorRef.current = {
        before_round_id: page.next_before_round_id ?? null,
        before_round_timestamp: page.next_before_round_timestamp,
      };
      setHasMoreHistory(page.has_more ?? false);
    }
    return true;
  } catch (err) {
    if (activeSessionKeyRef.current !== activeSessionKey) {
      return false;
    }
    console.error("[useAgentConversation] 加载目标轮次附近消息失败:", err);
    setError(
      err instanceof Error ? err.message : "Failed to load target messages",
    );
    return false;
  } finally {
    isRoundWindowLoadingRef.current = false;
  }
}
