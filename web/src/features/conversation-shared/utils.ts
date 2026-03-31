import { Message } from "@/types/message";

/** 将消息按 round_id 分组 */
export function groupMessagesByRound(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();
  for (const message of messages) {
    const round_id = message.round_id || message.message_id;
    if (!groups.has(round_id)) {
      groups.set(round_id, []);
    }
    groups.get(round_id)!.push(message);
  }
  return groups;
}

/** 获取最近一条 assistant/result 消息的时间戳 */
export function get_latest_reply_timestamp(messages: Message[]): number | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i];
    if (msg.role !== "assistant" && msg.role !== "result") continue;
    if (Number.isFinite(msg.timestamp) && msg.timestamp > 0) return msg.timestamp;
  }
  const last = messages[messages.length - 1];
  if (last && Number.isFinite(last.timestamp) && last.timestamp > 0) return last.timestamp;
  return null;
}
