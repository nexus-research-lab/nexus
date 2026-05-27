"use client";

import { useCallback, useEffect, useRef } from "react";

import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import {
  prepare_room_conversation_attachments,
  type PreparedComposerAttachment,
} from "@/features/conversation/shared/composer-attachments";

interface UseRoomComposerHandlersOptions {
  can_control_session: boolean;
  conversation_id: string | null;
  initial_draft?: string | null;
  is_loading: boolean;
  on_initial_draft_consumed?: () => void;
  room_id?: string | null;
  scroll_to_bottom: (behavior?: ScrollBehavior) => void;
  send_message: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
  session_key: string | null;
  try_handle_goal_command: (content: string) => Promise<boolean>;
}

export function useRoomComposerHandlers({
  can_control_session,
  conversation_id,
  initial_draft = null,
  is_loading,
  on_initial_draft_consumed,
  room_id = null,
  scroll_to_bottom,
  send_message,
  session_key,
  try_handle_goal_command,
}: UseRoomComposerHandlersOptions) {
  const consumed_initial_draft_ref = useRef<string | null>(null);

  const handle_send_message = useCallback(
    async (
      content: string,
      delivery_policy: AgentConversationDeliveryPolicy,
      attachments: PreparedComposerAttachment[] = [],
    ) => {
      if (!content.trim() && attachments.length === 0) return;
      if (await try_handle_goal_command(content)) {
        return;
      }
      scroll_to_bottom("auto");
      await send_message(content, { delivery_policy, attachments });
    },
    [scroll_to_bottom, send_message, try_handle_goal_command],
  );

  const handle_prepare_attachments = useCallback(
    async (files: File[]) => {
      if (!room_id || !conversation_id) {
        throw new Error("当前 Room 会话尚未就绪，暂时无法附加文件。");
      }
      return prepare_room_conversation_attachments(room_id, conversation_id, files);
    },
    [conversation_id, room_id],
  );

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (
      !session_key ||
      !normalized_draft ||
      is_loading ||
      !can_control_session
    ) {
      return;
    }

    const initial_draft_signature = `${session_key}:${normalized_draft}`;
    if (consumed_initial_draft_ref.current === initial_draft_signature) {
      return;
    }

    consumed_initial_draft_ref.current = initial_draft_signature;
    scroll_to_bottom("auto");
    void send_message(normalized_draft)
      .then(() => {
        on_initial_draft_consumed?.();
      })
      .catch((error) => {
        consumed_initial_draft_ref.current = null;
        console.error("Failed to auto send initial room prompt:", error);
      });
  }, [
    can_control_session,
    initial_draft,
    is_loading,
    on_initial_draft_consumed,
    scroll_to_bottom,
    send_message,
    session_key,
  ]);

  return {
    handle_prepare_attachments,
    handle_send_message,
  };
}
