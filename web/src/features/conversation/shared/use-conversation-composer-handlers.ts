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
  can_send_initial_draft: canSendInitialDraft = true,
  initial_draft: initialDraft = null,
  initial_draft_log_label: initialDraftLogLabel,
  is_loading: isLoading,
  on_initial_draft_consumed: onInitialDraftConsumed,
  prepare_attachments: prepareAttachments,
  scroll_to_bottom: scrollToBottom,
  send_message: sendMessage,
  session_key: sessionKey,
}: UseConversationComposerHandlersOptions) {
  const consumedInitialDraftRef = useRef<string | null>(null);

  const handleSendMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy,
      attachments: PreparedComposerAttachment[] = [],
    ) => {
      if (!content.trim() && attachments.length === 0) return;
      scrollToBottom("auto");
      await sendMessage(content, { delivery_policy: deliveryPolicy, attachments });
    },
    [scrollToBottom, sendMessage],
  );

  useEffect(() => {
    const normalizedDraft = initialDraft?.trim() ?? "";
    if (
      !sessionKey ||
      !normalizedDraft ||
      isLoading ||
      !canSendInitialDraft
    ) {
      return;
    }

    const initialDraftKey = `${sessionKey}:${normalizedDraft}`;
    if (consumedInitialDraftRef.current === initialDraftKey) {
      return;
    }

    consumedInitialDraftRef.current = initialDraftKey;
    scrollToBottom("auto");
    void sendMessage(normalizedDraft)
      .then(() => {
        onInitialDraftConsumed?.();
      })
      .catch((error) => {
        consumedInitialDraftRef.current = null;
        console.error(
          `Failed to auto send initial ${initialDraftLogLabel} prompt:`,
          error,
        );
      });
  }, [
    canSendInitialDraft,
    initialDraft,
    initialDraftLogLabel,
    isLoading,
    onInitialDraftConsumed,
    scrollToBottom,
    sendMessage,
    sessionKey,
  ]);

  return {
    handle_prepare_attachments: prepareAttachments,
    handle_send_message: handleSendMessage,
  };
}
