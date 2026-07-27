/**
 * INPUT: 当前 Room 或 DM Agent 身份，以及当前 Session 身份。
 * OUTPUT: 不含 Session 身份的逻辑聊天键，以及包含 Session 身份的完整草稿键。
 * POS: Room/DM 内隔离 Session 待发送状态、同时保持聊天级输入历史的唯一作用域规则。
 */

interface ComposerDraftScopeInput {
  agentId?: string | null;
  roomId?: string | null;
}

interface ComposerDraftRestoreInput {
  draftScopeKey: string;
  sessionKey?: string | null;
}

export function buildComposerDraftScopeKey({
  agentId,
  roomId,
}: ComposerDraftScopeInput): string {
  const normalizedRoomId = roomId?.trim();
  if (normalizedRoomId) {
    return `room:${normalizedRoomId}`;
  }
  const normalizedAgentId = agentId?.trim();
  return normalizedAgentId
    ? `agent:${normalizedAgentId}`
    : "conversation:unscoped";
}

export function buildComposerDraftRestoreKey({
  draftScopeKey,
  sessionKey,
}: ComposerDraftRestoreInput): string {
  const normalizedSessionKey = sessionKey?.trim();
  return normalizedSessionKey
    ? `${draftScopeKey}:session:${normalizedSessionKey}`
    : `${draftScopeKey}:session:unbound`;
}
