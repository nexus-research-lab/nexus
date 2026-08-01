import { formatRelativeTime } from "@/lib/format/relative-time";
import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";
import type { AgentPrivateThread } from "@/types/agent/private-domain";

export interface PrivateThreadListItemPresentation {
  buttonClassName: string;
  ownerAgentId: string;
  preview: string;
  scope: AgentPrivateThread["scope"];
  summaryClassName: string;
  thread: AgentPrivateThread;
  timestampLabel: string;
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
  summaryClassName: string;
  titleClassName: string;
}

const THREAD_DENSITY_PRESENTATIONS: Record<
  "compact" | "regular",
  PrivateThreadDensityPresentation
> = {
  compact: {
    activeClassName: SIDEBAR_SELECTION_CLASS_NAME,
    buttonClassName: "gap-2 rounded-[8px] px-2 py-2",
    containerClassName: "p-1.5",
    listClassName: "space-y-0.5",
    summaryClassName: "line-clamp-1 text-compact leading-4",
    titleClassName: "text-compact",
  },
  regular: {
    activeClassName: SIDEBAR_SELECTION_CLASS_NAME,
    buttonClassName: "gap-2.5 rounded-[8px] px-2.5 py-2.5",
    containerClassName: "p-2",
    listClassName: "space-y-0.5",
    summaryClassName: "line-clamp-1 text-xs leading-4",
    titleClassName: "text-compact",
  },
};

const IDLE_THREAD_CLASS_NAME =
  "border-transparent hover:bg-(--surface-interactive-hover-background)";

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
  const isActive = thread.thread_id === selectedThreadId;
  return {
    buttonClassName: cn(
      "group flex w-full min-w-0 items-start border text-left transition",
      density.buttonClassName,
      isActive ? density.activeClassName : IDLE_THREAD_CLASS_NAME,
    ),
    ownerAgentId: agentId,
    preview: thread.last_content_preview || "联络消息",
    scope: thread.scope,
    summaryClassName: cn(
      "mt-1 text-(--text-muted) [&_*]:leading-4",
      density.summaryClassName,
    ),
    thread,
    timestampLabel: thread.last_timestamp
      ? formatRelativeTime(thread.last_timestamp)
      : "",
    title: privateThreadTitle(thread, agentId),
    titleClassName: cn(
      "min-w-0 flex-1 truncate font-semibold text-(--text-strong)",
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
