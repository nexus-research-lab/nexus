/**
 * INPUT: 当前 Room 或 DM Agent 身份。
 * OUTPUT: 不含 Session 身份的 Composer 完整草稿作用域键，以及独立的恢复键。
 * POS: Room/DM 内跨 Session 共享完整输入、不同聊天隔离草稿的唯一作用域规则。
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
