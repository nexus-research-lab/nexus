/**
 * INPUT: Room root 消息、目标 Agent 与可选 agent_round_id。
 * OUTPUT: 目标执行轮的用户上下文、引导事件和 assistant 过程链。
 * POS: Room Agent Thread 的消息筛选真相源。
 */
import { isAutomationTriggerUserMessage } from "@/types/conversation/automation-message";
import type { Message } from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";

import { stripRoomControlMarkers } from "@/features/conversation/shared/message/message-content-model";

/** Thread 只展示全局输入、目标 Agent 的引导与该 Agent 的过程链。 */
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
  return messages
    .filter((message) => isRoomThreadMessage(
      message,
      agentId,
      normalizedAgentRoundId,
      hasExactAssistant,
    ))
    .map(projectRoomThreadMessage);
}

function isRoomThreadMessage(
  message: Message,
  agentId: string,
  agentRoundId: string | undefined,
  hasExactAssistant: boolean,
): boolean {
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
  if (message.role === "system") {
    return message.metadata?.subtype === "guided_input";
  }
  return !agentRoundId
    || message.agent_round_id?.trim() === agentRoundId
    || (!hasExactAssistant && !message.agent_round_id?.trim());
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
