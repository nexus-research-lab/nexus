import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type { Message } from "@/types/conversation/message/entity";

/** Thread 只展示用户输入与目标 Agent 的过程链，最终结果留在 Room 主时间线。 */
export function getRoomThreadMessages(
  messages: Message[],
  agentId: string,
): Message[] {
  return messages.filter(
    (message) =>
      (message.role === "user" && !isAutomationTriggerUserMessage(message)) ||
      (message.role === "system" &&
        message.agent_id === agentId &&
        message.metadata?.subtype === "guided_input") ||
      (message.role === "assistant" && message.agent_id === agentId),
  );
}
