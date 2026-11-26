// INPUT: 跨页面入口（首页启动台）在跳转前已上传完成的附件，以及目标会话消费初始草稿时的正文。
// OUTPUT: 与 `?initial=` 草稿正文一一对应、只能取走一次且会过期的附件交接。
// POS: initial 草稿协议的附件旁路；只存在当前页面内存，不持久化、不跨窗口。
import type { MessageAttachment } from "@/types/conversation/message/attachment";

/** 交接只覆盖一次跳转；超时未被消费说明目标会话没有接住，丢弃以免误附到之后的消息。 */
const HANDOFF_TTL_MS = 60_000;

interface InitialAttachmentHandoff {
  attachments: MessageAttachment[];
  expiresAt: number;
}

const handoffs = new Map<string, InitialAttachmentHandoff>();

export function stashInitialDraftAttachments(
  draft: string,
  attachments: MessageAttachment[],
): void {
  const key = draft.trim();
  if (!key || attachments.length === 0) {
    return;
  }
  handoffs.set(key, { attachments: [...attachments], expiresAt: Date.now() + HANDOFF_TTL_MS });
}

/** 按初始草稿正文取走附件；同一正文只交付一次。 */
export function takeInitialDraftAttachments(draft: string): MessageAttachment[] {
  const key = draft.trim();
  const handoff = handoffs.get(key);
  handoffs.delete(key);
  if (!handoff || handoff.expiresAt < Date.now()) {
    return [];
  }
  return handoff.attachments;
}
