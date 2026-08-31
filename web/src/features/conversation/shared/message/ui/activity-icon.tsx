import {
  BookOpen,
  Brain,
  Globe2,
  MousePointerClick,
  Pencil,
  Plug,
  Search,
  Send,
  Sparkles,
  SquareTerminal,
  Wrench,
  Workflow,
  type LucideIcon,
} from "lucide-react";

import type { ContentBlock } from "@/types/conversation/message/content";

import {
  resolveExecutionToolVisualKind,
  type ExecutionToolVisualKind,
} from "../../execution/execution-tool-visual";

const MAX_VISIBLE_ACTIVITY_ICONS = 6;
const TOOL_ICON_MAP: Readonly<Record<ExecutionToolVisualKind, LucideIcon>> = {
  browser: MousePointerClick,
  external: Plug,
  fetch: Globe2,
  generate: Sparkles,
  generic: Wrench,
  inspect: BookOpen,
  search: Search,
  send: Send,
  terminal: SquareTerminal,
  workflow: Workflow,
  write: Pencil,
};

interface ProcessActivityIcon {
  key: string;
  kind: "thinking" | ExecutionToolVisualKind;
}

export function ToolActivityIcon({
  className,
  kind,
}: {
  className: string;
  kind: ExecutionToolVisualKind;
}) {
  const Icon = TOOL_ICON_MAP[kind];
  return <Icon aria-hidden className={className} strokeWidth={1.8} />;
}

export function ProcessActivityIconStack({
  content,
}: {
  content: readonly ContentBlock[];
}) {
  const activities = content.flatMap<ProcessActivityIcon>((block, index) => {
    if (block.type === "thinking" && block.thinking.trim()) {
      return [{ key: `thinking:${index}`, kind: "thinking" as const }];
    }
    if (block.type === "tool_use") {
      return [{
        key: `tool:${block.id}`,
        kind: resolveExecutionToolVisualKind(block.name),
      }];
    }
    return [];
  });
  const visibleActivities = activities.slice(-MAX_VISIBLE_ACTIVITY_ICONS);
  const overflowCount = activities.length - visibleActivities.length;

  return (
    <span
      aria-hidden="true"
      className="flex shrink-0 items-center -space-x-1.5 pr-0.5"
      data-process-activity-icon-count={activities.length}
      data-process-activity-icon-stack
    >
      {visibleActivities.map((activity) => (
        <span
          className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-muted) shadow-(--surface-avatar-shadow)"
          data-process-activity-icon={activity.kind}
          key={activity.key}
        >
          {activity.kind === "thinking" ? (
            <Brain aria-hidden className="h-3 w-3" strokeWidth={1.8} />
          ) : (
            <ToolActivityIcon className="h-3 w-3" kind={activity.kind} />
          )}
        </span>
      ))}
      {overflowCount > 0 ? (
        <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[8px] font-semibold text-(--text-soft) shadow-(--surface-avatar-shadow)">
          +{overflowCount}
        </span>
      ) : null}
    </span>
  );
}
