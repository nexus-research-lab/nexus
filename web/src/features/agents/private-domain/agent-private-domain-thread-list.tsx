// INPUT: Agent 私域线程联合状态、选择真相与线程选择命令。
// OUTPUT: 共享 ListRow 驱动的线程目录，以及统一加载和空状态。
// POS: Agent 私域线程列表视图；字段投影归模型，时间线与请求生命周期归上层。

import {
  Inbox,
  Loader2,
  MessageCircle,
  StickyNote,
  UsersRound,
  type LucideIcon,
} from "lucide-react";

import { PrivateParticipantAvatarStack } from "@/features/agents/private-domain/agent-private-domain-avatar";
import {
  getPrivateThreadListLayout,
  type PrivateThreadListLayout,
} from "@/features/agents/private-domain/agent-private-domain-thread-layout";
import {
  getPrivateThreadListPresentation,
  type PrivateDomainLocalization,
  type PrivateThreadListItemPresentation,
  type PrivateThreadListPresentation,
} from "@/features/agents/private-domain/agent-private-domain-thread-model";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { AgentPrivateScope, AgentPrivateThread } from "@/types/agent/private-domain";

const THREAD_SCOPE_ICONS: Record<AgentPrivateScope, LucideIcon> = {
  audience: UsersRound,
  direct: MessageCircle,
  self: StickyNote,
};

export function PrivateThreadList({
  agentId,
  className,
  compact = false,
  isLoading,
  localization,
  onSelect,
  selectedThreadId,
  threads,
}: {
  agentId: string;
  className?: string;
  compact?: boolean;
  isLoading: boolean;
  localization: PrivateDomainLocalization;
  onSelect: (threadId: string) => void;
  selectedThreadId: string | null;
  threads: AgentPrivateThread[];
}) {
  const presentation = getPrivateThreadListPresentation({
    agentId,
    isLoading,
    localization,
    selectedThreadId,
    threads,
  });
  const layout = getPrivateThreadListLayout({
    className,
    compact,
    kind: presentation.kind,
  });
  return (
    <PrivateThreadListContent
      emptyLabel={localization.t("agent_options.contact.empty_records")}
      layout={layout}
      onSelect={onSelect}
      presentation={presentation}
    />
  );
}

function PrivateThreadListContent({
  emptyLabel,
  layout,
  onSelect,
  presentation,
}: {
  emptyLabel: string;
  layout: PrivateThreadListLayout;
  onSelect: (threadId: string) => void;
  presentation: PrivateThreadListPresentation;
}) {
  switch (presentation.kind) {
    case "loading":
      return (
        <div className={layout.containerClassName}>
          <Loader2
            className={getUiSpinnerClassName({ size: "lg", tone: "muted" })}
          />
        </div>
      );
    case "empty":
      return (
        <div className={layout.containerClassName}>
          <Inbox className="h-5 w-5 text-(--text-soft)" />
          <p className={getUiTypographyClassName({
            role: "metadata",
            tone: "muted",
            weight: "semibold",
          })}>{emptyLabel}</p>
        </div>
      );
    case "ready":
      return (
        <div className={layout.containerClassName}>
          <div className={layout.listClassName}>
            {presentation.items.map((item) => (
              <PrivateThreadListItem
                item={item}
                key={item.thread.thread_id}
                layout={layout}
                onSelect={onSelect}
              />
            ))}
          </div>
        </div>
      );
  }
}

function PrivateThreadListItem({
  item,
  layout,
  onSelect,
}: {
  item: PrivateThreadListItemPresentation;
  layout: PrivateThreadListLayout;
  onSelect: (threadId: string) => void;
}) {
  const ScopeIcon = THREAD_SCOPE_ICONS[item.scope];
  return (
    <UiListRow
      active={item.active}
      activeTone="sidebar"
      aria-pressed={item.active}
      className={layout.rowClassName}
      density={layout.density}
      onClick={() => onSelect(item.thread.thread_id)}
    >
      <PrivateParticipantAvatarStack
        ownerAgentId={item.ownerAgentId}
        participants={item.thread.participants}
      />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className={layout.titleClassName}>{item.title}</span>
          <ScopeIcon className="h-3.5 w-3.5 shrink-0 text-(--text-soft)" />
          {item.timestampLabel ? (
            <span className={layout.timestampClassName}>
              {item.timestampLabel}
            </span>
          ) : null}
        </div>
        <UiMarkdownContent
          className={layout.summaryClassName}
          content={item.preview}
          mermaidShowHeader={false}
          variant="summary"
          workspaceAgentId={item.workspaceAgentId}
        />
      </div>
    </UiListRow>
  );
}
