// INPUT: Agent 私域线程、当前 Agent/线程与本地化能力。
// OUTPUT: 加载联合状态和线程标题、摘要、Scope、时间等纯数据投影。
// POS: Agent 私域线程列表纯模型；不拥有布局、Typography、DOM 或交互样式。

import { formatRelativeTime } from "@/lib/format/relative-time";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { AgentPrivateThread } from "@/types/agent/private-domain";

export type PrivateDomainLocalization = Pick<I18nContextValue, "locale" | "t">;

export interface PrivateThreadListItemPresentation {
  active: boolean;
  ownerAgentId: string;
  preview: string;
  scope: AgentPrivateThread["scope"];
  thread: AgentPrivateThread;
  timestampLabel: string;
  title: string;
  workspaceAgentId: string;
}

export type PrivateThreadListPresentation =
  | { kind: "empty" }
  | { kind: "loading" }
  | {
      items: PrivateThreadListItemPresentation[];
      kind: "ready";
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
  localization: PrivateDomainLocalization,
): PrivateThreadListItemPresentation {
  const isActive = thread.thread_id === selectedThreadId;
  return {
    active: isActive,
    ownerAgentId: agentId,
    preview: thread.last_content_preview
      || localization.t("agent_options.contact.messages_title"),
    scope: thread.scope,
    thread,
    timestampLabel: thread.last_timestamp
      ? formatRelativeTime(thread.last_timestamp, localization.locale)
      : "",
    title: privateThreadTitle(thread, agentId, localization),
    workspaceAgentId: thread.participant_agent_ids[0] ?? agentId,
  };
}

export function getPrivateThreadListPresentation({
  agentId,
  isLoading,
  localization,
  selectedThreadId,
  threads,
}: {
  agentId: string;
  isLoading: boolean;
  localization: PrivateDomainLocalization;
  selectedThreadId: string | null;
  threads: AgentPrivateThread[];
}): PrivateThreadListPresentation {
  if (isLoading && threads.length === 0) {
    return { kind: "loading" };
  }
  if (threads.length === 0) {
    return { kind: "empty" };
  }

  return {
    items: threads.map((thread) => buildPrivateThreadListItem(
      thread,
      agentId,
      selectedThreadId,
      localization,
    )),
    kind: "ready",
  };
}
