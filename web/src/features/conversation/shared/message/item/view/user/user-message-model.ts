import { formatMessageTime } from "../../../message-time";
import type { UserMessage } from "@/types/conversation/message/entity";

interface UserMessageDensity {
  contentClassName: string;
  headerClassName: string;
  rowClassName: string;
  sectionClassName: string;
}

export interface UserMessagePresentation extends UserMessageDensity {
  displayContent: string;
  goal: boolean;
  guided: boolean;
  hasContent: boolean;
  timestamp: string;
}

const USER_MESSAGE_DENSITY: Record<"compact" | "expanded", UserMessageDensity> = {
  compact: {
    contentClassName: "text-base leading-6 [&_.katex-display]:my-2",
    headerClassName: "h-6",
    rowClassName: "",
    sectionClassName: "px-0",
  },
  expanded: {
    contentClassName: "text-[16px] leading-7 [&_.katex-display]:my-3",
    headerClassName: "h-7",
    rowClassName: "gap-3",
    sectionClassName: "px-2 sm:px-3",
  },
};

export function projectUserMessagePresentation(
  compact: boolean,
  content: string,
  message: UserMessage,
): UserMessagePresentation {
  const density = USER_MESSAGE_DENSITY[compact ? "compact" : "expanded"];
  const goal = message.metadata?.subtype === "goal_set";
  const displayContent = goal
    ? content.replace(/^\s*\/goal(?:\s+|$)/i, "").trim()
    : content;
  return {
    ...density,
    displayContent,
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
