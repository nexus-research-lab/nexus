/**
 * INPUT: 一组已确认的历史会话 ID、当前会话与可选的新会话准备命令。
 * OUTPUT: 先准备全量清空锚点，再串行删除并返回失败 ID 与新会话 ID。
 * POS: Room 历史批量删除事务适配层，保证当前会话最后删除且不维护展示状态。
 */

interface RoomHistoryBulkDeleteResult {
  failedConversationIds: string[];
  replacementConversationId: string | null;
}

interface RoomHistoryBulkDeleteOptions {
  currentConversationId?: string | null;
  createReplacementConversation?: () => Promise<string | null>;
}

function orderConversationIdsForDeletion(
  conversationIds: readonly string[],
  currentConversationId: string | null,
): string[] {
  const uniqueIds = [...new Set(conversationIds)];
  if (!currentConversationId || !uniqueIds.includes(currentConversationId)) {
    return uniqueIds;
  }
  return [
    ...uniqueIds.filter((id) => id !== currentConversationId),
    currentConversationId,
  ];
}

export async function deleteRoomHistoryConversationBatch(
  conversationIds: readonly string[],
  deleteConversation: (conversationId: string) => Promise<unknown>,
  {
    currentConversationId = null,
    createReplacementConversation,
  }: RoomHistoryBulkDeleteOptions = {},
): Promise<RoomHistoryBulkDeleteResult> {
  let replacementConversationId: string | null = null;
  if (createReplacementConversation) {
    try {
      replacementConversationId = await createReplacementConversation();
    } catch {
      return {
        failedConversationIds: [...new Set(conversationIds)],
        replacementConversationId: null,
      };
    }
    if (!replacementConversationId) {
      return {
        failedConversationIds: [...new Set(conversationIds)],
        replacementConversationId: null,
      };
    }
  }

  const failedConversationIds: string[] = [];
  const orderedConversationIds = orderConversationIdsForDeletion(
    conversationIds,
    currentConversationId,
  );
  for (const conversationId of orderedConversationIds) {
    try {
      await deleteConversation(conversationId);
    } catch {
      failedConversationIds.push(conversationId);
    }
  }
  return { failedConversationIds, replacementConversationId };
}
