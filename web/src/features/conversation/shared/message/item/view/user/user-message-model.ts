// INPUT: Durable user content, control-record metadata, delivery policy and timestamp.
// OUTPUT: Display content, Goal/guidance facts, time and available actions without CSS.
// POS: User-message semantic projection; reading geometry belongs to message-reading-layout.

import { formatMessageTime } from "../../../message-time";
import type { MessageExternalSource, UserMessage } from "@/types/conversation/message/entity";

export interface UserMessagePresentation {
  displayContent: string;
  externalSource: MessageExternalSource | null;
  goal: boolean;
  guided: boolean;
  hasContent: boolean;
  timestamp: string;
}

export function projectUserMessagePresentation(
  content: string,
  message: UserMessage,
): UserMessagePresentation {
  const goal = message.metadata?.subtype === "goal_set";
  const displayContent = goal
    ? content.replace(/^\s*\/goal(?:\s+|$)/i, "").trim()
    : content;
  return {
    displayContent,
    externalSource: message.external_source?.channel ? message.external_source : null,
    goal,
    guided: message.delivery_policy === "guide",
    hasContent: Boolean(displayContent),
    timestamp: formatMessageTime(message.timestamp),
  };
}

export function projectAvailableUserMessageAction<Action>(
  available: boolean,
  action: Action,
): Action | undefined {
  return available ? action : undefined;
}
