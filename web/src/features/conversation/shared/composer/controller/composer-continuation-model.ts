// INPUT: 当前会话消息的明确终态与 Room 当前成员身份。
// OUTPUT: 最后一条可见用户请求下仍中断的普通对话目标；不从正文猜测任务是否完成。
// POS: DM/Room 继续入口的纯消息投影，WorkGraph 资格仍由服务端 can_recover 决定。
import type { Message } from "@/types/conversation/message/entity";

export function resolveConversationContinuation(messages: readonly Message[], memberIds?: readonly string[]) {
  const visible = messages.filter((message) => !message.hidden_from_user && !message.is_synthetic && !message.parent_tool_use_id);
  const user = visible.findLast((message) => message.role === "user");
  if (!user) return null;
  const latest = new Map<string, Message>();
  for (const message of visible) {
    if (message.role === "assistant" && message.round_id === user.round_id) latest.set(message.agent_id, message);
  }
  const stopped = [...latest.values()].filter((message) => message.role === "assistant"
    && (message.result_summary?.subtype === "interrupted"
      || message.result_summary?.subtype === "error"
      || message.result_summary?.stop_reason === "max_tokens"
      || message.stop_reason === "max_tokens"
      || message.stream_status === "cancelled")
    && (!memberIds || memberIds.includes(message.agent_id)));
  if (!stopped.length) return null;
  return {
    agentRoundIds: stopped.map((message) => message.agent_round_id).filter((id): id is string => Boolean(id)),
    key: stopped.map((message) => message.message_id).sort().join("\u0000"),
    targetAgentIds: memberIds ? [...new Set(stopped.map((message) => message.agent_id))] : [],
  };
}
