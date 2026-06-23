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
  set_history_loading: (next_value: boolean) => void;
  set_has_more_history: (next_value: boolean) => void;
  set_history_prepend_token: Dispatch<SetStateAction<number>>;
  set_messages: Dispatch<SetStateAction<Message[]>>;
  set_error: Dispatch<SetStateAction<string | null>>;
}

export async function load_older_agent_conversation_messages({
  active_session_key_ref,
  identity,
  history_cursor_ref,
  has_more_history_ref,
  is_history_loading_ref,
  set_history_loading,
  set_has_more_history,
  set_history_prepend_token,
  set_messages,
  set_error,
}: LoadOlderAgentConversationMessagesParams): Promise<boolean> {
  const active_session_key = active_session_key_ref.current;
  const current_room_id = identity?.room_id?.trim() ?? "";
  const current_conversation_id = identity?.conversation_id?.trim() ?? "";
  const before_round_id = history_cursor_ref.current.before_round_id;
  const before_round_timestamp =
    history_cursor_ref.current.before_round_timestamp;

  if (
    !active_session_key ||
    !has_more_history_ref.current ||
    is_history_loading_ref.current ||
    !before_round_timestamp
  ) {
    return false;
  }

  set_history_loading(true);
  try {
    const page = current_room_id && current_conversation_id
      ? await get_room_conversation_messages(
          current_room_id,
          current_conversation_id,
          {
            limit: get_message_history_round_page_size(),
            before_round_id,
            before_round_timestamp,
          },
        )
      : await get_session_messages_api(active_session_key, {
          limit: get_message_history_round_page_size(),
          before_round_id,
          before_round_timestamp,
        });
    if (active_session_key_ref.current !== active_session_key) {
      return false;
    }

    const sorted_messages = sort_messages(page.items ?? []);
    if (sorted_messages.length === 0) {
      history_cursor_ref.current = {
        before_round_id: null,
        before_round_timestamp: null,
      };
      set_has_more_history(false);
      return false;
    }

    set_messages((current_messages) =>
      merge_loaded_messages(sorted_messages, current_messages),
    );
    history_cursor_ref.current = {
      before_round_id: page.next_before_round_id ?? null,
      before_round_timestamp: page.next_before_round_timestamp ?? null,
    };
    set_has_more_history(page.has_more ?? false);
    set_history_prepend_token((current_token) => current_token + 1);
    return true;
  } catch (err) {
    if (active_session_key_ref.current !== active_session_key) {
      return false;
    }
    console.error("[useAgentConversation] 加载更早消息失败:", err);
    set_error(
      err instanceof Error ? err.message : "Failed to load older messages",
    );
    return false;
  } finally {
    if (active_session_key_ref.current === active_session_key) {
      set_history_loading(false);
    }
  }
}
