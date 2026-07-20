import {
  MessageCircle,
  Trash2,
} from "lucide-react";

import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import { UiIconButton } from "@/shared/ui/button/button";
import { UiListRow } from "@/shared/ui/list/list-row";
import type { LauncherAgentSummary } from "@/types/app/launcher";

import type { SidebarConversationItem } from "./sidebar-conversation-model";

export function SidebarListLoadingRows({ count = 4 }: { count?: number }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2">
      {Array.from({ length: count }, (_, index) => (
        <div
          className="flex min-h-[54px] w-full items-center gap-2.5 rounded-[8px] px-2 py-1.5"
          key={index}
        >
          <span className="h-8 w-8 shrink-0 animate-pulse rounded-[9px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_74%,transparent)]" />
          <span className="min-w-0 flex-1 space-y-2">
            <span className="block h-3.5 w-24 animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_76%,transparent)]" />
            <span className="block h-3 w-36 animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)]" />
          </span>
        </div>
      ))}
    </div>
  );
}

interface ConversationRowProps {
  isActive: boolean;
  item: SidebarConversationItem;
  onClick: () => void;
  onDelete?: () => void;
}

function ConversationRowLeading({
  isWorking,
  item,
}: {
  isWorking: boolean;
  item: SidebarConversationItem;
}) {
  if (item.kind === "room") {
    return (
      <UiRoomAvatar
        avatar={item.avatar}
        members={item.members}
        roomId={item.roomId}
        size="sm"
        title={item.title}
      />
    );
  }
  return (
    <UiAgentAvatar
      avatar={(item.members[0]?.avatar ?? item.avatar) ?? undefined}
      isWorking={isWorking}
      name={item.members[0]?.name ?? item.title}
      size="sm"
    />
  );
}

function ConversationRowMeta({
  deleteLabel,
  onDelete,
  timeLabel,
}: {
  deleteLabel: string;
  onDelete?: () => void;
  timeLabel: string;
}) {
  if (!timeLabel && !onDelete) {
    return null;
  }
  return (
    <span className="relative flex h-7 w-10 shrink-0 items-center justify-end">
      {timeLabel ? (
        <span
          className={cn(
            "text-[11px] tabular-nums text-(--text-soft) transition-opacity duration-(--motion-duration-fast)",
            onDelete && "group-hover/item:opacity-0",
          )}
        >
          {timeLabel}
        </span>
      ) : null}
      {onDelete ? (
        <UiIconButton
          className="absolute right-0 top-1/2 -translate-y-1/2 opacity-0 group-hover/item:opacity-100"
          onClick={(event) => {
            event.stopPropagation();
            onDelete();
          }}
          size="sm"
          title={deleteLabel}
          tone="danger"
          type="button"
          variant="ghost"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </UiIconButton>
      ) : null}
    </span>
  );
}

function ConversationRowStatus({
  isWorking,
  unreadCount,
  workingLabel,
}: {
  isWorking: boolean;
  unreadCount: number;
  workingLabel: string;
}) {
  return (
    <>
      {isWorking ? (
        <UiBadge size="xs" tone="primary">
          {workingLabel}
        </UiBadge>
      ) : null}
      <UiCounterBadge count={unreadCount} />
    </>
  );
}

function ConversationRowSummary({ item }: { item: SidebarConversationItem }) {
  return (
    <UiMarkdownContent
      className="nexus-sidebar-conversation-summary truncate text-[12px] leading-5 text-(--text-muted) [&_*]:leading-5"
      content={item.summary}
      mermaidShowHeader={false}
      summaryMonochrome
      summaryStrongAsText
      variant="summary"
      workspaceAgentId={item.kind === "dm" ? item.agentId : undefined}
    />
  );
}

export function ConversationRow({
  item,
  isActive: isActive,
  onClick: onClick,
  onDelete: onDelete,
}: ConversationRowProps) {
  const { t } = useI18n();
  const isWorking = item.runningTaskCount > 0;

  return (
    <UiListRow
      active={isActive}
      className="min-h-[54px] gap-2.5 rounded-[8px] px-2 py-1.5"
      description={item.summary ? <ConversationRowSummary item={item} /> : undefined}
      leading={<ConversationRowLeading isWorking={isWorking} item={item} />}
      meta={item.timeLabel || onDelete ? (
        <ConversationRowMeta
          deleteLabel={t("common.delete")}
          onDelete={onDelete}
          timeLabel={item.timeLabel}
        />
      ) : null}
      onClick={onClick}
      subtitleTrailing={(
        <ConversationRowStatus
          isWorking={isWorking}
          unreadCount={item.unreadCount ?? 0}
          workingLabel={t("status.working")}
        />
      )}
      title={item.title}
    />
  );
}

export function ContactRow({
  agent,
  isActive: isActive,
  isWorking: isWorking,
  onChat: onChat,
  onOpenDirectory: onOpenDirectory,
  runningTaskCount: runningTaskCount,
}: {
  agent: LauncherAgentSummary;
  isActive: boolean;
  isWorking: boolean;
  onChat: () => void;
  onOpenDirectory: () => void;
  runningTaskCount: number;
}) {
  const { t } = useI18n();
  const description = agent.description?.trim();
  const subtitle = isWorking
    ? t("sidebar.running_tasks_short", { count: runningTaskCount })
    : (description || t("sidebar.contact_no_description"));
  const status = isWorking ? (
    <UiBadge size="xs" tone="primary">
      {t("status.working")}
    </UiBadge>
  ) : null;

  return (
    <UiListRow
      active={isActive}
      className="min-h-[54px] gap-2.5 rounded-[8px] px-2 py-1.5"
      description={subtitle}
      leading={<UiAgentAvatar avatar={agent.avatar} isWorking={isWorking} name={agent.name} size="sm" />}
      meta={status}
      onClick={onOpenDirectory}
      right={(
        <UiIconButton
          className="opacity-0 group-hover/item:opacity-100"
          onClick={(event) => {
            event.stopPropagation();
            onChat();
          }}
          title={t("sidebar.start_chat")}
          type="button"
          variant="ghost"
        >
          <MessageCircle className="h-4 w-4" />
        </UiIconButton>
      )}
      title={agent.name}
    />
  );
}
