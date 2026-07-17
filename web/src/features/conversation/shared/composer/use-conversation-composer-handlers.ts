import { useCallback, useEffect, useRef } from "react";

import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

interface UseConversationComposerHandlersOptions {
  canSendInitialDraft?: boolean;
  initialDraft?: string | null;
  initialDraftLogLabel: string;
  isLoading: boolean;
  onInitialDraftConsumed?: () => void;
  prepareAttachments: (files: File[]) => Promise<MessageAttachment[]>;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  sendMessage: (
    content: string,
    options?: AgentConversationSendOptions,
  ) => Promise<void>;
  sessionKey: string | null;
}

export function useConversationComposerHandlers({
  canSendInitialDraft = true,
  initialDraft = null,
  initialDraftLogLabel,
  isLoading,
  onInitialDraftConsumed,
  prepareAttachments,
  scrollToBottom,
  sendMessage,
  sessionKey,
}: UseConversationComposerHandlersOptions) {
  const consumedInitialDraftRef = useRef<string | null>(null);

  const handleSendMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy,
      attachments: MessageAttachment[] = [],
      targetAgentIDs: string[] = [],
    ) => {
      if (!content.trim() && attachments.length === 0) return;
      scrollToBottom("auto");
      await sendMessage(content, {
        delivery_policy: deliveryPolicy,
        attachments,
        target_agent_ids: targetAgentIDs,
      });
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
    handlePrepareAttachments: prepareAttachments,
    handleSendMessage,
  };
}
