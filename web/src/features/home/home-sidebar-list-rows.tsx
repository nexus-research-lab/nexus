import {
  MessageCircle,
  Trash2,
} from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/avatar";
import { UiBadge, UiCounterBadge } from "@/shared/ui/badge";
import { UiIconButton } from "@/shared/ui/button";
import { UiSearchInput } from "@/shared/ui/form-control";
import { UiListRow } from "@/shared/ui/list-row";
import type { LauncherAgentSummary } from "@/types/app/launcher";

import type { SidebarConversationItem } from "./home-sidebar-conversation-model";

export function SidebarSearchField({
  action,
  on_change: onChange,
  placeholder,
  value,
}: {
  action?: ReactNode;
  on_change: (value: string) => void;
  placeholder: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-2 px-2.5 pb-2">
      <UiSearchInput
        class_name="flex-1"
        input_class_name="text-[13px]"
        on_change={onChange}
        placeholder={placeholder}
        value={value}
      />
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

export function SidebarListLoadingRows({ count = 4 }: { count?: number }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-1 px-2 pb-2">
      {Array.from({ length: count }, (_, index) => (
        <div
          className="flex min-h-[68px] w-full items-center gap-3 rounded-[14px] px-3 py-2.5"
          key={index}
        >
          <span className="h-10 w-10 shrink-0 animate-pulse rounded-[10px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_74%,transparent)]" />
          <span className="min-w-0 flex-1 space-y-2">
            <span className="block h-3.5 w-24 animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_76%,transparent)]" />
            <span className="block h-3 w-36 animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)]" />
          </span>
        </div>
      ))}
    </div>
  );
}

export function ConversationRow({
  item,
  is_active: isActive,
  on_click: onClick,
  on_delete: onDelete,
}: {
  item: SidebarConversationItem;
  is_active: boolean;
  on_click: () => void;
  on_delete?: () => void;
}) {
  const { t } = useI18n();
  const isWorking = item.running_task_count > 0;
  const leading = item.kind === "room" ? (
    <UiRoomAvatar avatar={item.avatar} members={item.members} room_id={item.room_id} title={item.title} />
  ) : (
    <UiAgentAvatar
      avatar={(item.members[0]?.avatar ?? item.avatar) ?? undefined}
      is_working={isWorking}
      name={item.members[0]?.name ?? item.title}
    />
  );
  const meta = item.time_label || onDelete ? (
    <span className="relative flex h-7 w-10 shrink-0 items-center justify-end">
      {item.time_label ? (
        <span
          className={cn(
            "text-[11px] tabular-nums text-(--text-soft) transition-opacity duration-(--motion-duration-fast)",
            onDelete && "group-hover/item:opacity-0",
          )}
        >
          {item.time_label}
        </span>
      ) : null}
      {onDelete ? (
        <UiIconButton
          class_name="absolute right-0 top-1/2 -translate-y-1/2 opacity-0 group-hover/item:opacity-100"
          onClick={(event) => {
            event.stopPropagation();
            onDelete();
          }}
          size="sm"
          title={t("common.delete")}
          tone="danger"
          type="button"
          variant="ghost"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </UiIconButton>
      ) : null}
    </span>
  ) : null;
  const status = (
    <>
      {isWorking ? (
        <UiBadge size="xs" tone="primary">
          {t("status.working")}
        </UiBadge>
      ) : null}
      <UiCounterBadge count={item.unread_count ?? 0} />
    </>
  );

  return (
    <UiListRow
      active={isActive}
      description={item.summary}
      leading={leading}
      meta={meta}
      on_click={onClick}
      subtitle_trailing={status}
      title={item.title}
    />
  );
}

export function ContactRow({
  agent,
  is_active: isActive,
  is_working: isWorking,
  on_chat: onChat,
  on_open_directory: onOpenDirectory,
  running_task_count: runningTaskCount,
}: {
  agent: LauncherAgentSummary;
  is_active: boolean;
  is_working: boolean;
  on_chat: () => void;
  on_open_directory: () => void;
  running_task_count: number;
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
      description={subtitle}
      leading={<UiAgentAvatar avatar={agent.avatar} is_working={isWorking} name={agent.name} />}
      meta={status}
      on_click={onOpenDirectory}
      right={(
        <UiIconButton
          class_name="opacity-0 group-hover/item:opacity-100"
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
