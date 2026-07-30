/**
 * INPUT: 当前 Room 或 DM Agent 身份，以及当前 Session 身份。
 * OUTPUT: 按 Session 隔离的 Composer 草稿键，以及按逻辑聊天共享的发送历史键。
 * POS: Room/DM 内隔离 Session 待发送状态、同时保持聊天级发送历史的唯一作用域规则。
 */

interface ComposerDraftScopeInput {
  agentId?: string | null;
  roomId?: string | null;
  sessionKey?: string | null;
}

type ComposerChatScopeInput = Omit<ComposerDraftScopeInput, "sessionKey">;

export function buildComposerHistoryScopeKey({
  agentId,
  roomId,
}: ComposerChatScopeInput): string {
  const normalizedRoomId = roomId?.trim();
  if (normalizedRoomId) {
    return `room:${normalizedRoomId}`;
  }
  const normalizedAgentId = agentId?.trim();
  return normalizedAgentId
    ? `agent:${normalizedAgentId}`
    : "conversation:unscoped";
}

export function buildComposerDraftScopeKey({
  agentId,
  roomId,
  sessionKey,
}: ComposerDraftScopeInput): string {
  const historyScopeKey = buildComposerHistoryScopeKey({ agentId, roomId });
  const normalizedSessionKey = sessionKey?.trim();
  return normalizedSessionKey
    ? `${historyScopeKey}:session:${normalizedSessionKey}`
    : `${historyScopeKey}:session:unbound`;
}
