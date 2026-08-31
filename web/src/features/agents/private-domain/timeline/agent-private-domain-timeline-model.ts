/**
 * INPUT: 当前联络线程、消息快照和本地化能力。
 * OUTPUT: 线程标题与非失败正文的纯展示模型。
 * POS: 私域时间线展示规则；失败恢复由时间线组件单独承载。
 */
import { formatRelativeTime } from "@/lib/format/relative-time";
import type {
  AgentPrivateDirection,
  AgentPrivateEvent,
  AgentPrivateParticipant,
  AgentPrivateThread,
} from "@/types/agent/private-domain";
import type { RoomReplyRouteMode } from "@/types/agent/agent-conversation";

import {
  privateThreadTitle,
  type PrivateDomainLocalization,
} from "../agent-private-domain-thread-model";

export type PrivateTimelineDensity = "compact" | "regular";
export type PrivateTimelineBodyKind = "empty" | "events" | "select";

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
  events: AgentPrivateEvent[];
  isLoading: boolean;
  localization: PrivateDomainLocalization;
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
  localization: PrivateDomainLocalization,
): string {
  if (participantId === agentId) {
    return localization.t("agent_options.contact.self");
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
  localization: PrivateDomainLocalization,
): string[] {
  return recipientIds.map(
    (recipientId) => participantName(
      event,
      recipientId,
      agentId,
      localization,
    ),
  );
}

function formatParticipantNames(
  names: string[],
  localization: PrivateDomainLocalization,
): string {
  return names.join(localization.locale === "zh" ? "、" : ", ");
}

function privateReplyRouteLabel(
  event: AgentPrivateEvent,
  agentId: string,
  localization: PrivateDomainLocalization,
): string {
  const recipients = recipientNames(
    event,
    event.reply_route.recipients ?? [],
    agentId,
    localization,
  );
  return recipients.length > 0
    ? localization.t("agent_options.contact.reply_to", {
      names: formatParticipantNames(recipients, localization),
    })
    : localization.t("agent_options.contact.private_reply");
}

const REPLY_ROUTE_LABELS: Record<
  RoomReplyRouteMode,
  (
    event: AgentPrivateEvent,
    agentId: string,
    localization: PrivateDomainLocalization,
  ) => string
> = {
  none: (_event, _agentId, localization) => (
    localization.t("agent_options.contact.no_reply")
  ),
  private: privateReplyRouteLabel,
  public: (_event, _agentId, localization) => (
    localization.t("agent_options.contact.public_reply")
  ),
};

function eventRouteLabel(
  event: AgentPrivateEvent,
  agentId: string,
  localization: PrivateDomainLocalization,
): string {
  const recipients = recipientNames(
    event,
    event.recipients,
    agentId,
    localization,
  );
  if (recipients.length > 0) {
    return localization.t("agent_options.contact.to", {
      names: formatParticipantNames(recipients, localization),
    });
  }
  return REPLY_ROUTE_LABELS[event.reply_route.mode](
    event,
    agentId,
    localization,
  );
}

function eventSourceName(
  source: AgentPrivateParticipant | undefined,
  event: AgentPrivateEvent,
  agentId: string,
  localization: PrivateDomainLocalization,
): string {
  if (source?.agent_id === agentId) {
    return localization.t("agent_options.contact.self");
  }
  return source?.name || event.source_agent_id;
}

function buildEventPresentation(
  event: AgentPrivateEvent,
  agentId: string,
  localization: PrivateDomainLocalization,
): PrivateEventPresentation {
  const source = event.participants.find(
    (participant) => participant.agent_id === event.source_agent_id,
  );
  return {
    content: event.content || localization.t("agent_options.contact.empty_content"),
    direction: event.direction,
    id: event.message_id,
    routeLabel: eventRouteLabel(event, agentId, localization),
    source,
    sourceAgentId: event.source_agent_id,
    sourceName: eventSourceName(source, event, agentId, localization),
    timestampLabel: formatRelativeTime(event.timestamp, localization.locale),
  };
}

const TIMELINE_BODY_RULES: TimelineBodyRule[] = [
  {
    build: ({ localization }) => ({
      events: [],
      kind: "select",
      message: localization.t("agent_options.contact.select_record"),
    }),
    matches: ({ thread }) => !thread,
  },
  {
    build: ({ localization }) => ({
      events: [],
      kind: "empty",
      message: localization.t("agent_options.contact.empty_messages"),
    }),
    matches: ({ events, isLoading }) => events.length === 0 && !isLoading,
  },
];

const EVENTS_BODY_RULE: TimelineBodyRule = {
  build: ({ agentId, events, localization }) => ({
    events: events.map((event) => buildEventPresentation(
      event,
      agentId,
      localization,
    )),
    kind: "events",
    message: "",
  }),
  matches: () => true,
};

export function buildPrivateTimelineHeader(
  thread: AgentPrivateThread | null,
  agentId: string,
  localization: PrivateDomainLocalization,
): PrivateTimelineHeaderPresentation {
  if (!thread) {
    return {
      subtitle: null,
      title: localization.t("agent_options.contact.messages_title"),
    };
  }
  return {
    subtitle: `${thread.room_name || localization.t("agent_options.contact.default_room")} · ${thread.conversation_title || localization.t("agent_options.contact.default_conversation")}`,
    title: privateThreadTitle(thread, agentId, localization),
  };
}

export function buildPrivateTimelineBody(
  input: PrivateTimelineBodyInput,
): PrivateTimelineBodyPresentation {
  const rule = TIMELINE_BODY_RULES.find((candidate) => candidate.matches(input))
    ?? EVENTS_BODY_RULE;
  return rule.build(input);
}
