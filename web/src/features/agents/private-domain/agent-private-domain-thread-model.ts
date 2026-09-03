// INPUT: Agent 私域线程、当前 Agent/线程、显示密度与本地化能力。
// OUTPUT: 不含交互壳样式的加载状态和线程标题、摘要、Scope、时间展示投影。
// POS: Agent 私域线程列表纯模型；ListRow 拥有 DOM、键盘、焦点与选中视觉。

import { formatRelativeTime } from "@/lib/format/relative-time";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { UiListRowDensity } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { AgentPrivateThread } from "@/types/agent/private-domain";

export type PrivateDomainLocalization = Pick<I18nContextValue, "locale" | "t">;

export interface PrivateThreadListItemPresentation {
  active: boolean;
  density: UiListRowDensity;
  ownerAgentId: string;
  preview: string;
  rowClassName: string;
  scope: AgentPrivateThread["scope"];
  summaryClassName: string;
  thread: AgentPrivateThread;
  timestampLabel: string;
  timestampClassName: string;
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
  containerClassName: string;
  density: UiListRowDensity;
  listClassName: string;
  rowClassName: string;
  summaryClassName: string;
}

const THREAD_DENSITY_PRESENTATIONS: Record<
  "compact" | "regular",
  PrivateThreadDensityPresentation
> = {
  compact: {
    containerClassName: "p-1.5",
    density: "dense",
    listClassName: "space-y-0.5",
    rowClassName: "items-start gap-2",
    summaryClassName: "line-clamp-1 leading-4",
  },
  regular: {
    containerClassName: "p-2",
    density: "compact",
    listClassName: "space-y-0.5",
    rowClassName: "items-start gap-2.5",
    summaryClassName: "line-clamp-1 leading-4",
  },
};

export function privateThreadTitle(
  thread: AgentPrivateThread,
  agentId: string,
  localization: PrivateDomainLocalization,
): string {
  const peers = thread.participants.filter(
    (participant) => participant.agent_id !== agentId,
  );
  if (peers.length === 0) {
    return localization.t("agent_options.contact.private_note");
  }
  return peers
    .map((participant) => participant.name || participant.agent_id)
    .join(localization.locale === "zh" ? "、" : ", ");
}

function buildPrivateThreadListItem(
  thread: AgentPrivateThread,
  agentId: string,
  selectedThreadId: string | null,
  density: PrivateThreadDensityPresentation,
  localization: PrivateDomainLocalization,
): PrivateThreadListItemPresentation {
  const isActive = thread.thread_id === selectedThreadId;
  return {
    active: isActive,
    density: density.density,
    ownerAgentId: agentId,
    preview: thread.last_content_preview
      || localization.t("agent_options.contact.messages_title"),
    rowClassName: density.rowClassName,
    scope: thread.scope,
    summaryClassName: cn(
      "mt-1 [&_*]:leading-4",
      getUiTypographyClassName({ role: "metadata", tone: "muted" }),
      density.summaryClassName,
    ),
    thread,
    timestampLabel: thread.last_timestamp
      ? formatRelativeTime(thread.last_timestamp, localization.locale)
      : "",
    timestampClassName: cn(
      "ml-auto shrink-0 tabular-nums",
      getUiTypographyClassName({ role: "caption", tone: "soft" }),
    ),
    title: privateThreadTitle(thread, agentId, localization),
    titleClassName: cn(
      "min-w-0 flex-1 truncate",
      getUiTypographyClassName({
        role: "metadata",
        tone: "strong",
        weight: "semibold",
      }),
    ),
    workspaceAgentId: thread.participant_agent_ids[0] ?? agentId,
  };
}

export function getPrivateThreadListPresentation({
  agentId,
  className,
  compact,
  isLoading,
  localization,
  selectedThreadId,
  threads,
}: {
  agentId: string;
  className?: string;
  compact: boolean;
  isLoading: boolean;
  localization: PrivateDomainLocalization;
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
      localization,
    )),
    kind: "ready",
    listClassName: density.listClassName,
  };
}
