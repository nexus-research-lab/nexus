import { formatMessageTime } from "../../../message-time";
import type { UserMessage } from "@/types/conversation/message/entity";

interface UserMessageDensity {
  avatarFallbackClassName: string;
  avatarSize: "compact" | "full";
  contentClassName: string;
  headerClassName: string;
  rowClassName: string;
  sectionClassName: string;
}

export interface UserMessagePresentation extends UserMessageDensity {
  guided: boolean;
  hasContent: boolean;
  timestamp: string;
}

const USER_MESSAGE_DENSITY: Record<"compact" | "expanded", UserMessageDensity> = {
  compact: {
    avatarFallbackClassName: "h-3 w-3",
    avatarSize: "compact",
    contentClassName: "text-[15px] leading-6 [&_.katex-display]:my-2",
    headerClassName: "h-6",
    rowClassName: "",
    sectionClassName: "px-0",
  },
  expanded: {
    avatarFallbackClassName: "h-4 w-4",
    avatarSize: "full",
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
  return {
    ...density,
    guided: message.delivery_policy === "guide",
    hasContent: Boolean(content.trim()),
    timestamp: formatMessageTime(message.timestamp),
  };
}

export function projectAvailableUserMessageAction<Action>(
  available: boolean,
  action: Action,
): Action | undefined {
  return available ? action : undefined;
}
