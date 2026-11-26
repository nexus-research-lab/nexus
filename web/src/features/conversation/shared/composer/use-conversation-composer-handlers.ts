/**
 * INPUT: 会话草稿、附件准备、消息投递动作与滚动命令。
 * OUTPUT: 新 root 以 auto 进入真实内容底部的提交事务；初始草稿会一并带上入口交接的附件；smooth 只留给显式回到底部。
 * POS: DM/Room Composer 发送与初始草稿消费的交互编排层。
 */
import { useCallback, useEffect, useRef } from "react";

import type {
  AgentConversationDeliveryPolicy,
  AgentConversationSendOptions,
} from "@/types/agent/agent-conversation";
import type { MessageAttachment } from "@/types/conversation/message/attachment";

import {
  stashInitialDraftAttachments,
  takeInitialDraftAttachments,
} from "./composer-initial-attachments";

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
      // 中文注释：提交前直接回到真实底部，optimistic root 出现后由
      // FOLLOW 随每次内容增长同步上推；显式“回到底部”才使用平滑导航。
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
    const initialAttachments = takeInitialDraftAttachments(normalizedDraft);
    void sendMessage(
      normalizedDraft,
      initialAttachments.length > 0 ? { attachments: initialAttachments } : undefined,
    )
      .then(() => {
        onInitialDraftConsumed?.();
      })
      .catch((error) => {
        consumedInitialDraftRef.current = null;
        // 发送失败时归还附件，下次重试初始草稿仍带着它们。
        stashInitialDraftAttachments(normalizedDraft, initialAttachments);
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
