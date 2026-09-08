/**
 * INPUT: 最后一个已打开 Conversation、runtime close、draft ensure 与带作用域校验的提交命令。
 * OUTPUT: draft 会话先停后复用，已开始会话先切换后在后台停止的提交顺序。
 * POS: Room 最后标签的异步替换事务，不拥有具体 Store 或路由。
 */

import { isExternalSessionConversation } from "@/lib/conversation/external-session";
import type { RoomConversationView } from "@/types/conversation/conversation";

interface ReplaceFinalConversationOptions {
  closeConversation: (conversationId: string) => Promise<void>;
  commitConversation: (conversationId: string) => void;
  conversation: RoomConversationView;
  createConversation: () => Promise<string | null>;
  isCurrent: () => boolean;
}

export type FinalConversationReplacementHandler = (
  conversation: RoomConversationView,
  commitConversation: (conversationId: string) => void,
) => Promise<void>;

export async function replaceFinalConversation({
  closeConversation,
  commitConversation,
  conversation,
  createConversation,
  isCurrent,
}: ReplaceFinalConversationOptions): Promise<void> {
  if (!isCurrent()) {
    return;
  }

  const shouldCloseRuntime = !isExternalSessionConversation(conversation);
  const mustCloseBeforeCreate = shouldCloseRuntime && conversation.is_draft === true;
  if (mustCloseBeforeCreate) {
    try {
      await closeConversation(conversation.conversation_id);
    } catch {
      return;
    }
    if (!isCurrent()) {
      return;
    }
  }

  const replacementConversationId = await createConversation();
  if (!replacementConversationId || !isCurrent()) {
    return;
  }

  const unexpectedlyReusedStartedConversation = (
    shouldCloseRuntime
    && !mustCloseBeforeCreate
    && replacementConversationId === conversation.conversation_id
  );
  if (unexpectedlyReusedStartedConversation) {
    try {
      await closeConversation(conversation.conversation_id);
    } catch {
      return;
    }
    if (!isCurrent()) {
      return;
    }
  }

  commitConversation(replacementConversationId);

  if (
    shouldCloseRuntime
    && !mustCloseBeforeCreate
    && !unexpectedlyReusedStartedConversation
  ) {
    void closeConversation(conversation.conversation_id).catch(() => undefined);
  }
}
