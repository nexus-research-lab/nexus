// INPUT: Home 目录加载数量，以及已投影的会话、联系人、活动和操作数据。
// OUTPUT: 复用共享列表、身份、徽标、按钮与骨架原语的侧栏目录行。
// POS: Home sidebar 行级视图；不拥有基础组件视觉 recipe 或业务数据获取。

import {
  MessageCircle,
  Trash2,
} from "lucide-react";

import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import { UiSkeleton } from "@/shared/ui/display/skeleton";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import type { LauncherAgentSummary } from "@/types/app/launcher";

import type { SidebarConversationItem } from "./sidebar-conversation-model";

export function SidebarListLoadingRows({ count = 4 }: { count?: number }) {
  const { t } = useI18n();

  return (
    <div
      aria-busy="true"
      aria-label={t("common.loading")}
      className="flex min-h-0 flex-1 flex-col gap-0.5 px-2 pb-2 max-[559px]:gap-1 max-[559px]:px-3"
      role="status"
    >
      {Array.from({ length: count }, (_, index) => (
        <div
          className="flex min-h-[60px] w-full items-center gap-2.5 rounded-[8px] px-2 py-2 max-[559px]:min-h-[80px] max-[559px]:gap-3 max-[559px]:rounded-[12px] max-[559px]:px-3 max-[559px]:py-3"
          key={index}
        >
          <UiSkeleton className="h-10 w-10 shrink-0 radius-control-sm" tone="strong" />
          <span className="min-w-0 flex-1 space-y-2">
            <UiSkeleton className="h-3.5 w-24" tone="strong" />
            <UiSkeleton className="h-3 w-36" tone="subtle" />
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
  isActive,
  item,
}: {
  isActive: boolean;
  item: SidebarConversationItem;
}) {
  if (item.kind === "room") {
    return (
      <UiRoomAvatar
        avatar={item.avatar}
        members={item.members}
        roomId={item.roomId}
        size="md"
        title={item.title}
        isWorking={isActive}
      />
    );
  }
  return (
    <UiAgentAvatar
      avatar={(item.members[0]?.avatar ?? item.avatar) ?? undefined}
      isWorking={isActive}
      name={item.members[0]?.name ?? item.title}
      size="md"
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
    <span className="relative flex h-5 w-10 shrink-0 items-center justify-end">
      {timeLabel ? (
        <span
          className={cn(
            "text-xs tabular-nums text-(--text-soft) transition-opacity duration-(--motion-duration-fast)",
            onDelete && "group-hover/item:opacity-0 group-focus-within/item:opacity-0",
          )}
        >
          {timeLabel}
        </span>
      ) : null}
      {onDelete ? (
        <UiListActionButton
          className="absolute right-0 top-1/2 -translate-y-1/2"
          onClick={(event) => {
            event.stopPropagation();
            onDelete();
          }}
          size="sm"
          title={deleteLabel}
          tone="danger"
          type="button"
          visibility="hover"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </UiListActionButton>
      ) : null}
    </span>
  );
}

function ConversationRowStatus({
  activityStatus,
  unreadCount,
}: {
  activityStatus: SidebarConversationItem["activityStatus"];
  unreadCount: number;
}) {
  const { t } = useI18n();
  const presentation = activityStatus
    ? ACTIVITY_PRESENTATION[activityStatus]
    : null;
  return (
    <>
      {presentation ? (
        <UiBadge size="xs" tone={presentation.tone}>
          {t(presentation.labelKey)}
        </UiBadge>
      ) : null}
      <UiCounterBadge count={unreadCount} />
    </>
  );
}

function ConversationRowSummary({ item }: { item: SidebarConversationItem }) {
  return (
    <UiMarkdownContent
      className="nexus-sidebar-conversation-summary truncate text-compact leading-[1.125rem] text-(--text-soft) [&_*]:leading-[1.125rem]"
      content={item.summary}
      mermaidShowHeader={false}
      summaryMonochrome
      summaryStrongAsText
      variant="summary"
    />
  );
}

export function ConversationRow({
  item,
  isActive,
  onClick: onClick,
  onDelete: onDelete,
}: ConversationRowProps) {
  const { t } = useI18n();
  const hasActivity = item.activityStatus !== null;

  return (
    <UiListRow
      active={isActive}
      activeTone="sidebar"
      className="min-h-[60px] gap-2.5 rounded-[10px] px-2 py-2 max-[559px]:min-h-[80px] max-[559px]:gap-3 max-[559px]:rounded-[12px] max-[559px]:px-3 max-[559px]:py-3"
      description={item.summary ? <ConversationRowSummary item={item} /> : undefined}
      inactiveTone="muted"
      leading={<ConversationRowLeading isActive={hasActivity} item={item} />}
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
          activityStatus={item.activityStatus}
          unreadCount={item.unreadCount ?? 0}
        />
      )}
      title={item.title}
    />
  );
}

const ACTIVITY_PRESENTATION = {
  waiting: {
    labelKey: "status.needs_response",
    tone: "warning",
  },
  working: {
    labelKey: "status.working",
    tone: "running",
  },
} as const;

export function ContactRow({
  agent,
  isActive: isActive,
  onChat: onChat,
  onOpenDirectory: onOpenDirectory,
}: {
  agent: LauncherAgentSummary;
  isActive: boolean;
  onChat: () => void;
  onOpenDirectory: () => void;
}) {
  const { t } = useI18n();
  const description = agent.description?.trim();
  const subtitle = description || t("sidebar.contact_no_description");

  return (
    <UiListRow
      active={isActive}
      activeTone="sidebar"
      className="min-h-[54px] gap-2.5 rounded-[10px] py-1.5 pl-2 pr-[3px] max-[559px]:min-h-[72px] max-[559px]:gap-3 max-[559px]:rounded-[12px] max-[559px]:px-3 max-[559px]:py-2.5"
      description={subtitle}
      inactiveTone="muted"
      leading={(
        <UiAgentAvatar
          avatar={agent.avatar}
          name={agent.name}
          size="md"
        />
      )}
      onClick={onOpenDirectory}
      right={(
        <UiListActionButton
          onClick={(event) => {
            event.stopPropagation();
            onChat();
          }}
          title={t("sidebar.start_chat")}
          size="md"
          type="button"
          visibility="hover"
        >
          <MessageCircle className="h-[18px] w-[18px]" />
        </UiListActionButton>
      )}
      title={agent.name}
    />
  );
}
