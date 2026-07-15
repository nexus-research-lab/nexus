import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type { Message } from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";

import { stripRoomControlMarkers } from "@/features/conversation/shared/message/message-content-model";

/** Thread 只展示全局输入、目标 Agent 的引导与该 Agent 的过程链。 */
export function getRoomThreadMessages(
  messages: Message[],
  agentId: string,
): Message[] {
  return messages
    .filter((message) => isRoomThreadMessage(message, agentId))
    .map(projectRoomThreadMessage);
}

function isRoomThreadMessage(message: Message, agentId: string): boolean {
  if (message.role === "user") {
    if (isAutomationTriggerUserMessage(message)) {
      return false;
    }
    return message.delivery_policy !== "guide" ||
      message.target_agent_ids?.includes(agentId) === true;
  }

  if (message.agent_id !== agentId) {
    return false;
  }
  return message.role === "assistant" ||
    (message.role === "system" && message.metadata?.subtype === "guided_input");
}

function projectRoomThreadMessage(message: Message): Message {
	if (message.role === "assistant") {
		return {
			...message,
			content: message.content.map(stripContentBlockRoomMarkers),
		};
	}

	return {
		...message,
		content: stripRoomControlMarkers(message.content),
	};
}

function stripContentBlockRoomMarkers(block: ContentBlock): ContentBlock {
  if (block.type === "text") {
    return { ...block, text: stripRoomControlMarkers(block.text) };
  }
  if (block.type === "thinking") {
    return { ...block, thinking: stripRoomControlMarkers(block.thinking) };
  }
  if (block.type === "system_event") {
    return {
      ...block,
      content: stripRoomControlMarkers(block.content),
      label: stripRoomControlMarkers(block.label),
    };
  }
  return block;
}
