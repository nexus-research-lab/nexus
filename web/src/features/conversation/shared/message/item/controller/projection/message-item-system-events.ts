import type {
  SystemEventContent,
  SystemEventIcon,
} from "@/types/conversation/message/content";
import type {
  Message,
  SystemMessage,
} from "@/types/conversation/message/entity";

interface SystemMessageDisplayMeta {
  icon: SystemEventIcon;
  label: string;
  tone: "neutral" | "warning";
}

interface SystemEventVisibilityContext {
  includeTransientEvents: boolean;
  message: SystemMessage & { content: string };
}

const DEFAULT_SYSTEM_MESSAGE_DISPLAY_META: SystemMessageDisplayMeta = {
  icon: "status",
  label: "系统事件",
  tone: "neutral",
};

const SYSTEM_MESSAGE_DISPLAY_META_BY_SUBTYPE: Readonly<Record<
  string,
  SystemMessageDisplayMeta
>> = {
  api_retry: { icon: "retry", label: "自动重试", tone: "warning" },
  compact_boundary: { icon: "status", label: "上下文压缩", tone: "neutral" },
  guided_input: { icon: "guide", label: "已引导对话", tone: "neutral" },
  status: { icon: "status", label: "状态更新", tone: "neutral" },
  task_notification: { icon: "status", label: "状态更新", tone: "neutral" },
  task_progress: { icon: "progress", label: "执行状态", tone: "neutral" },
  task_started: { icon: "progress", label: "执行状态", tone: "neutral" },
  task_updated: { icon: "status", label: "状态更新", tone: "neutral" },
};

const SYSTEM_EVENT_VISIBILITY_RULES: ReadonlyArray<
  (context: SystemEventVisibilityContext) => boolean
> = [
  ({ message }) => Boolean(message.content.trim()),
  ({ includeTransientEvents, message }) => includeTransientEvents
    || message.metadata?.subtype === "guided_input",
];

export function buildSystemEventBlocks(
  messages: readonly Message[],
  includeTransientEvents: boolean,
): SystemEventContent[] {
  return messages.flatMap((message) => {
    const block = projectSystemEventBlock(message, includeTransientEvents);
    return block ? [block] : [];
  });
}

function projectSystemEventBlock(
  message: Message,
  includeTransientEvents: boolean,
): SystemEventContent | null {
  const systemMessage = getTextSystemMessage(message);
  if (!systemMessage) {
    return null;
  }
  const visibilityContext = { includeTransientEvents, message: systemMessage };
  if (!SYSTEM_EVENT_VISIBILITY_RULES.every((rule) => rule(visibilityContext))) {
    return null;
  }

  const displayMeta = getSystemMessageDisplayMeta(systemMessage);
  return {
    type: "system_event",
    content: systemMessage.content,
    icon: displayMeta.icon,
    label: displayMeta.label,
    source_message_id: systemMessage.message_id,
    subtype: systemMessage.metadata?.subtype,
    timestamp: systemMessage.timestamp,
    tone: displayMeta.tone,
    tool_use_id: getSystemEventToolUseId(systemMessage),
  };
}

function getTextSystemMessage(
  message: Message,
): (SystemMessage & { content: string }) | null {
  if (message.role !== "system" || typeof message.content !== "string") {
    return null;
  }
  return message;
}

function getSystemMessageDisplayMeta(
  message: SystemMessage,
): SystemMessageDisplayMeta {
  return SYSTEM_MESSAGE_DISPLAY_META_BY_SUBTYPE[message.metadata?.subtype ?? ""]
    ?? DEFAULT_SYSTEM_MESSAGE_DISPLAY_META;
}

function getSystemEventToolUseId(message: SystemMessage): string | null {
  return typeof message.metadata?.tool_use_id === "string"
    ? message.metadata.tool_use_id
    : null;
}
