import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { cn } from "@/shared/ui/class-name";

import { PrivateParticipantAvatar } from "../agent-private-domain-avatar";
import type {
  PrivateEventPresentation,
  PrivateTimelineDensity,
} from "./agent-private-domain-timeline-model";

interface DirectionStyle {
  alignment: string;
  bubble: string;
}

interface DensityStyle {
  bubble: string;
  content: string;
  header: string;
  name: string;
  route: string;
}

const DIRECTION_STYLES: Record<
  PrivateEventPresentation["direction"],
  DirectionStyle
> = {
  incoming: {
    alignment: "justify-start",
    bubble: "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_62%,transparent)]",
  },
  outgoing: {
    alignment: "justify-end",
    bubble: "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]",
  },
  self: {
    alignment: "justify-center",
    bubble: "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_72%,transparent)]",
  },
};

const DENSITY_STYLES: Record<PrivateTimelineDensity, DensityStyle> = {
  compact: {
    bubble: "max-w-[88%] rounded-[13px] px-2.5 py-2 shadow-none",
    content: "mt-1.5 text-[12.5px] leading-5",
    header: "gap-1.5",
    name: "text-[11.5px]",
    route: "mt-1.5 text-[10px]",
  },
  regular: {
    bubble: "max-w-[min(720px,78%)] rounded-[16px] px-3 py-2.5 shadow-[0_8px_24px_rgba(15,23,42,0.04)]",
    content: "mt-2 text-[13px] leading-5",
    header: "gap-2",
    name: "text-[12px]",
    route: "mt-2 text-[10.5px]",
  },
};

export function PrivateEventBubble({
  density,
  event,
}: {
  density: PrivateTimelineDensity;
  event: PrivateEventPresentation;
}) {
  const direction = DIRECTION_STYLES[event.direction];
  const size = DENSITY_STYLES[density];
  return (
    <div className={cn("flex", direction.alignment)}>
      <div className={cn("w-fit border", size.bubble, direction.bubble)}>
        <div className={cn("flex min-w-0 items-center", size.header)}>
          <PrivateParticipantAvatar participant={event.source} size="sm" />
          <span className={cn("truncate font-bold text-(--text-strong)", size.name)}>
            {event.sourceName}
          </span>
          <span className="rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_68%,transparent)] px-1.5 py-0.5 text-[10px] font-semibold text-(--text-soft)">
            私信
          </span>
          <span className="ml-auto shrink-0 text-[10.5px] font-semibold text-(--text-soft)">
            {event.timestampLabel}
          </span>
        </div>
        <UiMarkdownContent
          className={cn(
            "text-(--text-default) [&_[data-markdown-anchor]]:my-1 [&_[data-markdown-anchor]]:leading-5 [&_blockquote]:my-2 [&_ol]:mb-2 [&_ol]:space-y-1 [&_ul]:mb-2 [&_ul]:space-y-1",
            size.content,
          )}
          content={event.content}
          mermaidShowHeader={false}
          workspaceAgentId={event.sourceAgentId}
        />
        <p className={cn("truncate font-semibold text-(--text-soft)", size.route)}>
          {event.routeLabel}
        </p>
      </div>
    </div>
  );
}
