/**
 * INPUT: transport/history 中不可信的 public handoff reply annotation。
 * OUTPUT: 三个精确身份字段均有效的展示级回执，或 null。
 * POS: 消息协议与 Room 回执展示之间的纯校验边界；不产生 mention 或 wake 动作。
 */
import type { PublicHandoffReply } from "@/types/conversation/message/entity";

export function normalizePublicHandoffReply(
  value: unknown,
): PublicHandoffReply | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const candidate = value as Partial<Record<keyof PublicHandoffReply, unknown>>;
  const handoffId = normalizeIdentity(candidate.handoff_id);
  const sourceMessageId = normalizeIdentity(candidate.source_message_id);
  const sourceAgentId = normalizeIdentity(candidate.source_agent_id);
  if (!handoffId || !sourceMessageId || !sourceAgentId) {
    return null;
  }
  return {
    handoff_id: handoffId,
    source_message_id: sourceMessageId,
    source_agent_id: sourceAgentId,
  };
}

function normalizeIdentity(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}
