// INPUT: 私域事件投影、密度与事件来源 Agent 的文件资源。
// OUTPUT: 保留 exact 来源文件预览、公共元信息排版与独立正文密度的消息气泡。
// POS: 私域时间线消费侧；方向色与正文密度属于消息阅读几何，共享 Markdown 不解析业务身份。
import { useWorkspaceMarkdown } from "@/hooks/agent/use-workspace-markdown";

import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
  header: string;
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
    bubble: "max-w-[88%] px-2.5 py-2",
    header: "gap-1.5",
  },
  regular: {
    bubble: "max-w-[min(760px,82%)] px-3 py-2.5",
    header: "gap-2",
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
    <div className={cn("flex", direction.alignment)} data-private-event={event.id}>
      <div className={cn("surface-radius-md w-fit", size.bubble, direction.bubble)}>
        <div className={cn("flex min-w-0 items-center", size.header)}>
          <PrivateParticipantAvatar participant={event.source} size="sm" />
          <span className={cn("truncate", getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "semibold" }))}>
            {event.sourceName}
          </span>
          <span className={cn("ml-auto shrink-0 tabular-nums", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
            {event.timestampLabel}
          </span>
        </div>
        <UiMarkdownContent
          className={cn(
            "text-(--text-default) [&_[data-markdown-anchor]]:my-1 [&_[data-markdown-anchor]]:leading-5 [&_blockquote]:my-2 [&_ol]:mb-2 [&_ol]:space-y-1 [&_ul]:mb-2 [&_ul]:space-y-1",
            "mt-1.5 text-sm leading-5",
          )}
          content={event.content}
          mermaidShowHeader={false}
          getFilePreviewUrl={getFilePreviewUrl}
          resolveFilePath={resolveFilePath}
        />
        <p className={cn("mt-1.5 truncate", getUiTypographyClassName({ role: "caption", tone: "soft", weight: "medium" }))}>
          {event.routeLabel}
        </p>
      </div>
    </div>
  );
}
