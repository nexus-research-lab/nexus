/**
 * INPUT: 已确认的联络消息快照、读取失败事实与重试命令。
 * OUTPUT: 可保留旧消息的 Problem/Impact/Recovery 时间线状态。
 * POS: 私域消息纯展示边界；不发起读取或推断业务结果。
 */
import {
  Inbox,
  Loader2,
  MessageCircle,
  RefreshCw,
  type LucideIcon,
} from "lucide-react";
import { type ComponentType } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type {
  AgentPrivateEvent,
  AgentPrivateThread,
} from "@/types/agent/private-domain";

import type { PrivateDomainLocalization } from "../agent-private-domain-thread-model";
import { PrivateEventBubble } from "./agent-private-domain-event";
import {
  buildPrivateTimelineBody,
  buildPrivateTimelineHeader,
  type PrivateTimelineBodyKind,
  type PrivateTimelineBodyPresentation,
  type PrivateTimelineDensity,
} from "./agent-private-domain-timeline-model";

interface PrivateTimelineProps {
  agentId: string;
  className?: string;
  compact?: boolean;
  failure: PrivateDomainReadFailure | null;
  events: AgentPrivateEvent[];
  isLoading: boolean;
  localization: PrivateDomainLocalization;
  onRetry: () => void;
  thread: AgentPrivateThread | null;
}

export interface PrivateDomainReadFailure {
  message: string;
  stale: boolean;
}

interface TimelineDensityStyle {
  body: string;
  header: string;
  headerFrame: string;
  section: string;
  subtitle: string;
  title: string;
}

interface TimelineBodyViewProps {
  density: PrivateTimelineDensity;
  presentation: PrivateTimelineBodyPresentation;
}

const TIMELINE_DENSITY_STYLES: Record<
  PrivateTimelineDensity,
  TimelineDensityStyle
> = {
  compact: {
    body: "min-h-full px-3 py-3",
    header: "h-10 px-3",
    headerFrame: "border-b border-(--divider-subtle-color)",
    section: "surface-radius-md border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_30%,transparent)]",
    subtitle: "text-2xs",
    title: "text-sm",
  },
  regular: {
    body: "mx-auto min-h-full w-full max-w-[920px] px-6 py-4 max-sm:px-4",
    header: "mx-auto min-h-[48px] w-full max-w-[920px] px-6 py-2 max-sm:px-4",
    headerFrame: "",
    section: "nexus-private-domain-reader",
    subtitle: "text-xs",
    title: "text-sm",
  },
};

function EmptyTimelineBody({
  icon: Icon,
  message,
}: {
  icon: LucideIcon;
  message: string;
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 text-(--text-soft)">
      <Icon className="h-6 w-6" />
      <span className="text-compact font-semibold">{message}</span>
    </div>
  );
}

function SelectTimelineBody({ presentation }: TimelineBodyViewProps) {
  return <EmptyTimelineBody icon={MessageCircle} message={presentation.message} />;
}

function NoEventsTimelineBody({ presentation }: TimelineBodyViewProps) {
  return <EmptyTimelineBody icon={Inbox} message={presentation.message} />;
}

function EventsTimelineBody({
  density,
  presentation,
}: TimelineBodyViewProps) {
  return (
    <div className="space-y-2.5">
      {presentation.events.map((event) => (
        <PrivateEventBubble density={density} event={event} key={event.id} />
      ))}
    </div>
  );
}

const TIMELINE_BODY_VIEWS: Record<
  PrivateTimelineBodyKind,
  ComponentType<TimelineBodyViewProps>
> = {
  empty: NoEventsTimelineBody,
  events: EventsTimelineBody,
  select: SelectTimelineBody,
};

function PrivateTimelineBody({
  density,
  presentation,
}: TimelineBodyViewProps) {
  const Body = TIMELINE_BODY_VIEWS[presentation.kind];
  return <Body density={density} presentation={presentation} />;
}

export function PrivateEventTimeline({
  agentId,
  className,
  compact = false,
  failure,
  events,
  isLoading,
  localization,
  onRetry,
  thread,
}: PrivateTimelineProps) {
  const density: PrivateTimelineDensity = compact ? "compact" : "regular";
  const style = TIMELINE_DENSITY_STYLES[density];
  const header = buildPrivateTimelineHeader(thread, agentId, localization);
  const body = buildPrivateTimelineBody({
    agentId,
    events,
    isLoading,
    localization,
    thread,
  });

  return (
    <section
      className={cn(
        "flex min-h-0 min-w-0 flex-col overflow-hidden",
        style.section,
        className,
      )}
    >
      <div className={style.headerFrame}>
        <div className={cn(
          "flex items-center justify-between gap-3",
          style.header,
        )}>
          <div className="min-w-0">
            <p className={cn("truncate font-semibold text-(--text-strong)", style.title)}>
              {header.title}
            </p>
            {header.subtitle ? (
              <p className={cn("mt-0.5 truncate font-medium text-(--text-soft)", style.subtitle)}>
                {header.subtitle}
              </p>
            ) : null}
          </div>
          {isLoading ? (
            <Loader2 className="h-4 w-4 animate-spin text-(--text-soft)" />
          ) : null}
        </div>
      </div>
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto">
        <div className={cn(style.body, "space-y-3")}>
          {failure ? (
            <UiResourceState
              className="min-h-0 py-3"
              description={failure.message}
              impact={failure.stale
                ? localization.t("agent_options.contact.private_messages_stale_impact")
                : localization.t("agent_options.contact.private_messages_unavailable_impact")}
              nextStep={localization.t("agent_options.contact.private_messages_failure_next_step")}
              primaryAction={{
                busy: isLoading,
                busyLabel: localization.t("common.loading"),
                icon: <RefreshCw className="h-3.5 w-3.5" />,
                label: localization.t("agent_options.contact.retry_private_messages"),
                onClick: onRetry,
              }}
              size="sm"
              state="error"
              title={localization.t("agent_options.contact.private_messages_load_failed")}
            />
          ) : null}
          {failure && !failure.stale && events.length === 0 ? null : (
            <PrivateTimelineBody density={density} presentation={body} />
          )}
        </div>
      </div>
    </section>
  );
}
