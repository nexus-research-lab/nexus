/**
 * INPUT: 一组已确认的历史会话 ID 与既有单会话删除命令。
 * OUTPUT: 串行执行全部删除并返回失败 ID，单项失败不阻断后续项。
 * POS: Room 历史批量删除事务适配层，不维护选择或展示状态。
 */

interface RoomHistoryBulkDeleteResult {
  failedConversationIds: string[];
}

export async function deleteRoomHistoryConversationBatch(
  conversationIds: readonly string[],
  deleteConversation: (conversationId: string) => Promise<unknown>,
): Promise<RoomHistoryBulkDeleteResult> {
  const failedConversationIds: string[] = [];
  for (const conversationId of conversationIds) {
    try {
      await deleteConversation(conversationId);
    } catch {
      failedConversationIds.push(conversationId);
    }
  }
  return { failedConversationIds };
}
