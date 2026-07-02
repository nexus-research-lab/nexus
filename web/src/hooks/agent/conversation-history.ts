import { Dispatch, MutableRefObject, RefObject, SetStateAction } from "react";
import { get_message_history_round_page_size } from "@/config/options";
import { get_session_messages_api } from "@/lib/api/agent-api";
import { get_room_conversation_messages } from "@/lib/api/room-api";
import { Message } from "@/types";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { merge_loaded_messages, sort_messages } from "./message-helpers";

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

export async function load_older_agent_conversation_messages({
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
      ? await get_room_conversation_messages(
          currentRoomId,
          currentConversationId,
          {
            limit: get_message_history_round_page_size(),
            before_round_id: beforeRoundId,
            before_round_timestamp: beforeRoundTimestamp,
          },
        )
      : await get_session_messages_api(activeSessionKey, {
          limit: get_message_history_round_page_size(),
          before_round_id: beforeRoundId,
          before_round_timestamp: beforeRoundTimestamp,
        });
    if (activeSessionKeyRef.current !== activeSessionKey) {
      return false;
    }

    const sortedMessages = sort_messages(page.items ?? []);
    if (sortedMessages.length === 0) {
      historyCursorRef.current = {
        before_round_id: null,
        before_round_timestamp: null,
      };
      setHasMoreHistory(false);
      return false;
    }

    setMessages((currentMessages) =>
      merge_loaded_messages(sortedMessages, currentMessages),
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
