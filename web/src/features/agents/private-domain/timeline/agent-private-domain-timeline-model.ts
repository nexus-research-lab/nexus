import { formatRelativeTime } from "@/lib/format/relative-time";
import type {
  AgentPrivateDirection,
  AgentPrivateEvent,
  AgentPrivateParticipant,
  AgentPrivateThread,
} from "@/types/agent/private-domain";
import type { RoomReplyRouteMode } from "@/types/agent/agent-conversation";

import { privateThreadTitle } from "../agent-private-domain-thread-model";

export type PrivateTimelineDensity = "compact" | "regular";
export type PrivateTimelineBodyKind = "empty" | "error" | "events" | "select";

export interface PrivateEventPresentation {
  content: string;
  direction: AgentPrivateDirection;
  id: string;
  routeLabel: string;
  source: AgentPrivateParticipant | undefined;
  sourceAgentId: string;
  sourceName: string;
  timestampLabel: string;
}

export interface PrivateTimelineHeaderPresentation {
  subtitle: string | null;
  title: string;
}

export interface PrivateTimelineBodyPresentation {
  events: PrivateEventPresentation[];
  kind: PrivateTimelineBodyKind;
  message: string;
}

interface PrivateTimelineBodyInput {
  agentId: string;
  error: string | null;
  events: AgentPrivateEvent[];
  isLoading: boolean;
  thread: AgentPrivateThread | null;
}

interface TimelineBodyRule {
  build: (input: PrivateTimelineBodyInput) => PrivateTimelineBodyPresentation;
  matches: (input: PrivateTimelineBodyInput) => boolean;
}

function participantName(
  event: AgentPrivateEvent,
  participantId: string,
  agentId: string,
): string {
  if (participantId === agentId) {
    return "我";
  }
  const participant = event.participants.find(
    (item) => item.agent_id === participantId,
  );
  return participant?.name || participantId;
}

function recipientNames(
  event: AgentPrivateEvent,
  recipientIds: string[],
  agentId: string,
): string[] {
  return recipientIds.map(
    (recipientId) => participantName(event, recipientId, agentId),
  );
}

function privateReplyRouteLabel(
  event: AgentPrivateEvent,
  agentId: string,
): string {
  const recipients = recipientNames(
    event,
    event.reply_route.recipients ?? [],
    agentId,
  );
  return recipients.length > 0
    ? `回复到 ${recipients.join("、")}`
    : "私密回复";
}

const REPLY_ROUTE_LABELS: Record<
  RoomReplyRouteMode,
  (event: AgentPrivateEvent, agentId: string) => string
> = {
  none: () => "不要求回复",
  private: privateReplyRouteLabel,
  public: () => "回复到公区",
};

function eventRouteLabel(event: AgentPrivateEvent, agentId: string): string {
  const recipients = recipientNames(event, event.recipients, agentId);
  if (recipients.length > 0) {
    return `给 ${recipients.join("、")}`;
  }
  return REPLY_ROUTE_LABELS[event.reply_route.mode](event, agentId);
}

function eventSourceName(
  source: AgentPrivateParticipant | undefined,
  event: AgentPrivateEvent,
  agentId: string,
): string {
  if (source?.agent_id === agentId) {
    return "我";
  }
  return source?.name || event.source_agent_id;
}

function buildEventPresentation(
  event: AgentPrivateEvent,
  agentId: string,
): PrivateEventPresentation {
  const source = event.participants.find(
    (participant) => participant.agent_id === event.source_agent_id,
  );
  return {
    content: event.content || "（无正文）",
    direction: event.direction,
    id: event.message_id,
    routeLabel: eventRouteLabel(event, agentId),
    source,
    sourceAgentId: event.source_agent_id,
    sourceName: eventSourceName(source, event, agentId),
    timestampLabel: formatRelativeTime(event.timestamp),
  };
}

const TIMELINE_BODY_RULES: TimelineBodyRule[] = [
  {
    build: ({ error }) => ({ events: [], kind: "error", message: error || "" }),
    matches: ({ error }) => Boolean(error),
  },
  {
    build: () => ({ events: [], kind: "select", message: "选择一条联络记录" }),
    matches: ({ thread }) => !thread,
  },
  {
    build: () => ({ events: [], kind: "empty", message: "暂无消息" }),
    matches: ({ events, isLoading }) => events.length === 0 && !isLoading,
  },
];

const EVENTS_BODY_RULE: TimelineBodyRule = {
  build: ({ agentId, events }) => ({
    events: events.map((event) => buildEventPresentation(event, agentId)),
    kind: "events",
    message: "",
  }),
  matches: () => true,
};

export function buildPrivateTimelineHeader(
  thread: AgentPrivateThread | null,
  agentId: string,
): PrivateTimelineHeaderPresentation {
  if (!thread) {
    return { subtitle: null, title: "联络消息" };
  }
  return {
    subtitle: `${thread.room_name || "房间"} · ${thread.conversation_title || "主对话"}`,
    title: privateThreadTitle(thread, agentId),
  };
}

export function buildPrivateTimelineBody(
  input: PrivateTimelineBodyInput,
): PrivateTimelineBodyPresentation {
  const rule = TIMELINE_BODY_RULES.find((candidate) => candidate.matches(input))
    ?? EVENTS_BODY_RULE;
  return rule.build(input);
}
