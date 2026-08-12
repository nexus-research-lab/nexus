/**
 * INPUT: 最后一个已打开 Conversation、runtime close、draft ensure 与带作用域校验的提交命令。
 * OUTPUT: draft 会话先停后复用，已开始会话先切换后在后台停止的替换结果。
 * POS: Workspace 最后标签的异步替换事务，不拥有具体 Store 或路由。
 */

import { isExternalSessionConversation } from "@/lib/conversation/external-session";
import type { RoomConversationView } from "@/types/conversation/conversation";

interface ReplaceFinalConversationOptions {
  closeConversation?: (conversationId: string) => Promise<void>;
  commitConversation: (conversationId: string) => boolean;
  conversation: RoomConversationView;
  createConversation: () => Promise<string | null>;
  isCurrent: () => boolean;
}

export async function replaceFinalConversation({
  closeConversation,
  commitConversation,
  conversation,
  createConversation,
  isCurrent,
}: ReplaceFinalConversationOptions): Promise<string | null> {
  if (!isCurrent()) {
    return null;
  }

  const shouldCloseRuntime = Boolean(
    closeConversation && !isExternalSessionConversation(conversation),
  );
  const mustCloseBeforeCreate = shouldCloseRuntime && conversation.is_draft === true;
  if (mustCloseBeforeCreate && closeConversation) {
    try {
      await closeConversation(conversation.conversation_id);
    } catch {
      return null;
    }
    if (!isCurrent()) {
      return null;
    }
  }

  const replacementConversationId = await createConversation();
  if (!replacementConversationId || !isCurrent()) {
    return null;
  }

  const unexpectedlyReusedStartedConversation = (
    shouldCloseRuntime
    && !mustCloseBeforeCreate
    && replacementConversationId === conversation.conversation_id
  );
  if (unexpectedlyReusedStartedConversation && closeConversation) {
    try {
      await closeConversation(conversation.conversation_id);
    } catch {
      return null;
    }
    if (!isCurrent()) {
      return null;
    }
  }

  if (!commitConversation(replacementConversationId)) {
    return null;
  }

  if (
    shouldCloseRuntime
    && !mustCloseBeforeCreate
    && !unexpectedlyReusedStartedConversation
    && closeConversation
  ) {
    void closeConversation(conversation.conversation_id).catch(() => undefined);
  }
  return replacementConversationId;
}
