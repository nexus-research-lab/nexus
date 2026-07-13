import { formatRelativeTime } from "@/lib/format/relative-time";
import { cn } from "@/shared/ui/class-name";
import type { AgentPrivateThread } from "@/types/agent/private-domain";

export interface PrivateThreadListItemPresentation {
  buttonClassName: string;
  metadata: string[];
  metadataClassName: string;
  ownerAgentId: string;
  preview: string;
  scope: AgentPrivateThread["scope"];
  summaryClassName: string;
  thread: AgentPrivateThread;
  title: string;
  titleClassName: string;
  workspaceAgentId: string;
}

export type PrivateThreadListPresentation =
  | { className: string; kind: "empty" }
  | { className: string; kind: "loading" }
  | {
      className: string;
      items: PrivateThreadListItemPresentation[];
      kind: "ready";
      listClassName: string;
    };

interface PrivateThreadDensityPresentation {
  activeClassName: string;
  buttonClassName: string;
  containerClassName: string;
  listClassName: string;
  metadataClassName: string;
  summaryClassName: string;
  titleClassName: string;
}

const THREAD_DENSITY_PRESENTATIONS: Record<
  "compact" | "regular",
  PrivateThreadDensityPresentation
> = {
  compact: {
    activeClassName: "border-transparent bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] shadow-[inset_2px_0_0_var(--primary)]",
    buttonClassName: "gap-2 rounded-[10px] px-2 py-2",
    containerClassName: "p-1.5",
    listClassName: "space-y-0.5",
    metadataClassName: "mt-1 text-[10px]",
    summaryClassName: "line-clamp-1 text-[11.5px] leading-4",
    titleClassName: "text-[12.5px]",
  },
  regular: {
    activeClassName: "border-[color:color-mix(in_srgb,var(--primary)_38%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)]",
    buttonClassName: "gap-2.5 rounded-[12px] px-2.5 py-2.5",
    containerClassName: "p-2",
    listClassName: "space-y-1",
    metadataClassName: "mt-1.5 text-[10.5px]",
    summaryClassName: "line-clamp-2 text-[12px] leading-4",
    titleClassName: "text-[13px]",
  },
};

const IDLE_THREAD_CLASS_NAME =
  "border-transparent hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background)";

export function privateThreadTitle(
  thread: AgentPrivateThread,
  agentId: string,
): string {
  const peers = thread.participants.filter(
    (participant) => participant.agent_id !== agentId,
  );
  if (peers.length === 0) {
    return "私有笔记";
  }
  return peers
    .map((participant) => participant.name || participant.agent_id)
    .join("、");
}

function buildPrivateThreadListItem(
  thread: AgentPrivateThread,
  agentId: string,
  selectedThreadId: string | null,
  density: PrivateThreadDensityPresentation,
): PrivateThreadListItemPresentation {
  const metadata = [thread.room_name || "房间", String(thread.message_count)];
  if (thread.last_timestamp) {
    metadata.push(formatRelativeTime(thread.last_timestamp));
  }
  const isActive = thread.thread_id === selectedThreadId;
  return {
    buttonClassName: cn(
      "group flex w-full min-w-0 items-start border text-left transition",
      density.buttonClassName,
      isActive ? density.activeClassName : IDLE_THREAD_CLASS_NAME,
    ),
    metadata,
    metadataClassName: cn(
      "flex items-center font-semibold text-(--text-soft) [&>span+span]:before:mx-1.5 [&>span+span]:before:content-['·']",
      density.metadataClassName,
    ),
    ownerAgentId: agentId,
    preview: thread.last_content_preview || "联络消息",
    scope: thread.scope,
    summaryClassName: cn(
      "mt-1 text-(--text-muted) [&_*]:leading-4",
      density.summaryClassName,
    ),
    thread,
    title: privateThreadTitle(thread, agentId),
    titleClassName: cn(
      "truncate font-bold text-(--text-strong)",
      density.titleClassName,
    ),
    workspaceAgentId: thread.participant_agent_ids[0] ?? agentId,
  };
}

export function getPrivateThreadListPresentation({
  agentId,
  className,
  compact,
  isLoading,
  selectedThreadId,
  threads,
}: {
  agentId: string;
  className?: string;
  compact: boolean;
  isLoading: boolean;
  selectedThreadId: string | null;
  threads: AgentPrivateThread[];
}): PrivateThreadListPresentation {
  if (isLoading && threads.length === 0) {
    return {
      className: cn("flex items-center justify-center", className),
      kind: "loading",
    };
  }
  if (threads.length === 0) {
    return {
      className: cn(
        "flex flex-col items-center justify-center gap-2 px-4 text-center",
        className,
      ),
      kind: "empty",
    };
  }

  const density = THREAD_DENSITY_PRESENTATIONS[compact ? "compact" : "regular"];
  return {
    className: cn(
      "soft-scrollbar min-h-0 overflow-y-auto",
      density.containerClassName,
      className,
    ),
    items: threads.map((thread) => buildPrivateThreadListItem(
      thread,
      agentId,
      selectedThreadId,
      density,
    )),
    kind: "ready",
    listClassName: density.listClassName,
  };
}
