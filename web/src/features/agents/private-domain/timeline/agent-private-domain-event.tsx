// INPUT: 私域事件投影、密度与事件来源 Agent 的文件资源。
// OUTPUT: 保留 exact 来源文件预览的消息气泡。
// POS: 私域时间线消费侧；共享 Markdown 不解析业务身份。
import { useWorkspaceMarkdown } from "@/hooks/agent/use-workspace-markdown";

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
    bubble: "bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_62%,transparent)]",
  },
  outgoing: {
    alignment: "justify-end",
    bubble: "bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)]",
  },
  self: {
    alignment: "justify-center",
    bubble: "bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_72%,transparent)]",
  },
};

const DENSITY_STYLES: Record<PrivateTimelineDensity, DensityStyle> = {
  compact: {
    bubble: "max-w-[88%] radius-control-lg px-2.5 py-2",
    content: "mt-1.5 text-sm leading-5",
    header: "gap-1.5",
    name: "text-compact",
    route: "mt-1.5 text-2xs",
  },
  regular: {
    bubble: "max-w-[min(760px,82%)] rounded-[12px] px-3 py-2.5",
    content: "mt-1.5 text-sm leading-5",
    header: "gap-2",
    name: "text-compact",
    route: "mt-1.5 text-2xs",
  },
};

export function PrivateEventBubble({
  density,
  event,
}: {
  density: PrivateTimelineDensity;
  event: PrivateEventPresentation;
}) {
  const { resolveFilePath, getFilePreviewUrl } = useWorkspaceMarkdown(event.sourceAgentId);
  const direction = DIRECTION_STYLES[event.direction];
  const size = DENSITY_STYLES[density];
  return (
    <div className={cn("flex", direction.alignment)}>
      <div className={cn("w-fit", size.bubble, direction.bubble)}>
        <div className={cn("flex min-w-0 items-center", size.header)}>
          <PrivateParticipantAvatar participant={event.source} size="sm" />
          <span className={cn("truncate font-semibold text-(--text-strong)", size.name)}>
            {event.sourceName}
          </span>
          <span className="ml-auto shrink-0 text-2xs tabular-nums text-(--text-soft)">
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
          getFilePreviewUrl={getFilePreviewUrl}
          resolveFilePath={resolveFilePath}
        />
        <p className={cn("truncate font-semibold text-(--text-soft)", size.route)}>
          {event.routeLabel}
        </p>
      </div>
    </div>
  );
}
