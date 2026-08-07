import {
  Inbox,
  Loader2,
  MessageCircle,
  type LucideIcon,
} from "lucide-react";
import { type ComponentType } from "react";

import { cn } from "@/shared/ui/class-name";
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
  error: string | null;
  events: AgentPrivateEvent[];
  isLoading: boolean;
  localization: PrivateDomainLocalization;
  thread: AgentPrivateThread | null;
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

function ErrorTimelineBody({ presentation }: TimelineBodyViewProps) {
  return (
    <p className="surface-radius-md border border-[color:color-mix(in_srgb,var(--destructive)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-3 py-2 text-compact font-semibold text-(--destructive)">
      {presentation.message}
    </p>
  );
}

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
  error: ErrorTimelineBody,
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
  error,
  events,
  isLoading,
  localization,
  thread,
}: PrivateTimelineProps) {
  const density: PrivateTimelineDensity = compact ? "compact" : "regular";
  const style = TIMELINE_DENSITY_STYLES[density];
  const header = buildPrivateTimelineHeader(thread, agentId, localization);
  const body = buildPrivateTimelineBody({
    agentId,
    error,
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
        <div className={style.body}>
          <PrivateTimelineBody density={density} presentation={body} />
        </div>
      </div>
    </section>
  );
}
