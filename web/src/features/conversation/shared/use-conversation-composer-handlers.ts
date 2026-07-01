import { useCallback, useEffect, useRef } from "react";

import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";

import type { PreparedComposerAttachment } from "./composer-attachments";

interface UseConversationComposerHandlersOptions {
  can_send_initial_draft?: boolean;
  initial_draft?: string | null;
  initial_draft_log_label: string;
  is_loading: boolean;
  on_initial_draft_consumed?: () => void;
  prepare_attachments: (files: File[]) => Promise<PreparedComposerAttachment[]>;
  scroll_to_bottom: (behavior?: ScrollBehavior) => void;
  send_message: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
  session_key: string | null;
}

export function useConversationComposerHandlers({
  can_send_initial_draft = true,
  initial_draft = null,
  initial_draft_log_label,
  is_loading,
  on_initial_draft_consumed,
  prepare_attachments,
  scroll_to_bottom,
  send_message,
  session_key,
}: UseConversationComposerHandlersOptions) {
  const consumed_initial_draft_ref = useRef<string | null>(null);

  const handle_send_message = useCallback(
    async (
      content: string,
      delivery_policy: AgentConversationDeliveryPolicy,
      attachments: PreparedComposerAttachment[] = [],
    ) => {
      if (!content.trim() && attachments.length === 0) return;
      scroll_to_bottom("auto");
      await send_message(content, { delivery_policy, attachments });
    },
    [scroll_to_bottom, send_message],
  );

  useEffect(() => {
    const normalized_draft = initial_draft?.trim() ?? "";
    if (
      !session_key ||
      !normalized_draft ||
      is_loading ||
      !can_send_initial_draft
    ) {
      return;
    }

    const initial_draft_key = `${session_key}:${normalized_draft}`;
    if (consumed_initial_draft_ref.current === initial_draft_key) {
      return;
    }

    consumed_initial_draft_ref.current = initial_draft_key;
    scroll_to_bottom("auto");
    void send_message(normalized_draft)
      .then(() => {
        on_initial_draft_consumed?.();
      })
      .catch((error) => {
        consumed_initial_draft_ref.current = null;
        console.error(
          `Failed to auto send initial ${initial_draft_log_label} prompt:`,
          error,
        );
      });
  }, [
    can_send_initial_draft,
    initial_draft,
    initial_draft_log_label,
    is_loading,
    on_initial_draft_consumed,
    scroll_to_bottom,
    send_message,
    session_key,
  ]);

  return {
    handle_prepare_attachments: prepare_attachments,
    handle_send_message,
  };
}
