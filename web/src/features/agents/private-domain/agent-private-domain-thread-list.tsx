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
  getPrivateThreadListPresentation,
  type PrivateDomainLocalization,
  type PrivateThreadListItemPresentation,
  type PrivateThreadListPresentation,
} from "@/features/agents/private-domain/agent-private-domain-thread-model";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
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
    className,
    compact,
    isLoading,
    localization,
    selectedThreadId,
    threads,
  });
  return (
    <PrivateThreadListContent
      emptyLabel={localization.t("agent_options.contact.empty_records")}
      onSelect={onSelect}
      presentation={presentation}
    />
  );
}

function PrivateThreadListContent({
  emptyLabel,
  onSelect,
  presentation,
}: {
  emptyLabel: string;
  onSelect: (threadId: string) => void;
  presentation: PrivateThreadListPresentation;
}) {
  switch (presentation.kind) {
    case "loading":
      return (
        <div className={presentation.className}>
          <Loader2
            className={getUiSpinnerClassName({ size: "lg", tone: "muted" })}
          />
        </div>
      );
    case "empty":
      return (
        <div className={presentation.className}>
          <Inbox className="h-5 w-5 text-(--text-soft)" />
          <p className="text-compact font-semibold text-(--text-muted)">{emptyLabel}</p>
        </div>
      );
    case "ready":
      return (
        <div className={presentation.className}>
          <div className={presentation.listClassName}>
            {presentation.items.map((item) => (
              <PrivateThreadListItem
                item={item}
                key={item.thread.thread_id}
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
  onSelect,
}: {
  item: PrivateThreadListItemPresentation;
  onSelect: (threadId: string) => void;
}) {
  const ScopeIcon = THREAD_SCOPE_ICONS[item.scope];
  return (
    <button
      className={item.buttonClassName}
      onClick={() => onSelect(item.thread.thread_id)}
      type="button"
    >
      <PrivateParticipantAvatarStack
        ownerAgentId={item.ownerAgentId}
        participants={item.thread.participants}
      />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className={item.titleClassName}>{item.title}</span>
          <ScopeIcon className="h-3.5 w-3.5 shrink-0 text-(--text-soft)" />
          {item.timestampLabel ? (
            <span className="ml-auto shrink-0 text-2xs tabular-nums text-(--text-soft)">
              {item.timestampLabel}
            </span>
          ) : null}
        </div>
        <UiMarkdownContent
          className={item.summaryClassName}
          content={item.preview}
          mermaidShowHeader={false}
          variant="summary"
          workspaceAgentId={item.workspaceAgentId}
        />
      </div>
    </button>
  );
}
