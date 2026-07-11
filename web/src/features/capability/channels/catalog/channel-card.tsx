import {
  Clock3,
  ExternalLink,
  Send,
  Settings2,
  UserRound,
  UsersRound,
} from "lucide-react";

import { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { cn } from "@/lib/utils";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import { ChannelIcon } from "../channel-icon";
import { isChannelPlanned } from "../channel-model";
import { describeChannel } from "./channel-catalog-model";

function ChannelStatPill({
  icon: Icon,
  label,
  value,
  tone = "default",
}: {
  icon: typeof Send;
  label: string;
  value: number | string;
  tone?: "default" | "warning";
}) {
  return (
    <span
      className={cn(
        "inline-flex h-6 shrink-0 items-center gap-1 rounded-[7px] border px-2 text-[11px] font-semibold leading-none",
        tone === "warning"
          ? "border-[color:color-mix(in_srgb,var(--warning)_30%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] text-(--text-strong)"
          : "border-(--divider-subtle-color) bg-(--surface-elevated-background) text-(--text-muted)",
      )}
    >
      <Icon className="h-3.5 w-3.5 text-(--icon-muted)" />
      <span>{label}</span>
      <span className="tabular-nums text-(--text-strong)">{value}</span>
    </span>
  );
}

export function ChannelCard({
  item,
  onConfigure,
}: {
  item: ChannelConfigView;
  onConfigure: (item: ChannelConfigView) => void;
}) {
  const planned = isChannelPlanned(item);
  const description = describeChannel(item);
  const handlerLabel = item.configured ? item.agent_name || "已绑定" : "未绑定";

  return (
    <UiListRow
      className={cn(
        "min-h-[72px] rounded-[14px] px-2 py-1.5",
        planned && "cursor-default opacity-70",
      )}
      leading={<ChannelIcon type={item.channel_type} />}
      onClick={planned ? undefined : () => onConfigure(item)}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          {!planned && item.docs_url ? (
            <UiListActionButton
              onClick={() => window.open(item.docs_url, "_blank", "noopener,noreferrer")}
              size="sm"
              stopPropagation
              title="查看接入文档"
            >
              <ExternalLink className="h-3 w-3" />
            </UiListActionButton>
          ) : null}
          {!planned ? (
            <UiListActionButton
              className="text-(--primary)"
              onClick={() => onConfigure(item)}
              size="sm"
              stopPropagation
              title="设置机器人"
              visibility="visible"
            >
              <Settings2 className="h-3 w-3" />
            </UiListActionButton>
          ) : (
            <span className="flex h-8 w-8 items-center justify-center text-(--icon-muted)">
              <Clock3 className="h-3.5 w-3.5" />
            </span>
          )}
        </div>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {item.title}
          </span>
          {item.runtime_status === "external_adapter" ? (
            <UiBadge size="xs" tone="warning">外部适配器</UiBadge>
          ) : null}
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {description}
        </div>
        <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] leading-4 text-(--text-soft)">
          <span className="min-w-0 truncate">机器人：{item.bot_label}</span>
          <span className="min-w-0 truncate">处理：{handlerLabel}</span>
          {!item.supports_group ? <span className="shrink-0">仅私聊</span> : null}
        </div>
        <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5">
          <ChannelStatPill icon={UserRound} label="用户" value={item.stats.paired_user_count} />
          {item.supports_group ? (
            <ChannelStatPill icon={UsersRound} label="群聊" value={item.stats.paired_group_count} />
          ) : null}
          <ChannelStatPill
            icon={Clock3}
            label="待审"
            tone={item.stats.pending_count > 0 ? "warning" : "default"}
            value={item.stats.pending_count}
          />
        </div>
        {item.runtime_note ? (
          <div className="mt-0.5 truncate text-[11px] leading-4 text-(--text-soft)">
            {item.runtime_note}
          </div>
        ) : null}
      </div>
    </UiListRow>
  );
}
