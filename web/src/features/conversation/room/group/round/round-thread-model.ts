/**
 * INPUT: Room root 消息、目标 Agent 与可选 agent_round_id。
 * OUTPUT: 目标执行轮的用户上下文、引导事件和 assistant 过程链。
 * POS: Room Agent Thread 的消息筛选真相源。
 */
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type { Message } from "@/types/conversation/message/entity";

/** Thread 只展示用户输入与目标 Agent 的过程链，最终结果留在 Room 主时间线。 */
export function getRoomThreadMessages(
  messages: Message[],
  agentId: string,
  agentRoundId?: string | null,
): Message[] {
  const normalizedAgentRoundId = agentRoundId?.trim();
  const hasExactAssistant = Boolean(normalizedAgentRoundId) && messages.some(
    (message) => message.role === "assistant"
      && message.agent_id === agentId
      && message.agent_round_id?.trim() === normalizedAgentRoundId,
  );
  return messages.filter(
    (message) =>
      (message.role === "user" && !isAutomationTriggerUserMessage(message)) ||
      (message.role === "system" &&
        message.agent_id === agentId &&
        message.metadata?.subtype === "guided_input") ||
      (message.role === "assistant"
        && message.agent_id === agentId
        && (!normalizedAgentRoundId
          || message.agent_round_id?.trim() === normalizedAgentRoundId
          || (!hasExactAssistant && !message.agent_round_id?.trim()))),
  );
}
