import {
  Clock3,
  ExternalLink,
  Send,
  Settings2,
  UserRound,
  UsersRound,
} from "lucide-react";

import { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import { ChannelIcon } from "../channel-icon";
import {
  buildChannelCardModel,
  type ChannelCardModel,
  type ChannelCardStatIcon,
} from "./channel-catalog-model";

const CHANNEL_STAT_ICONS: Record<ChannelCardStatIcon, typeof Send> = {
  group: UsersRound,
  pending: Clock3,
  user: UserRound,
};

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

function ChannelCardActions({
  action,
  onConfigure,
}: {
  action: ChannelCardModel["action"];
  onConfigure: () => void;
}) {
  if (action.kind === "planned") {
    return (
      <span className="flex h-8 w-8 items-center justify-center text-(--icon-muted)">
        <Clock3 className="h-3.5 w-3.5" />
      </span>
    );
  }

  return (
    <div className="flex shrink-0 items-center gap-1.5">
      {action.docsUrl ? (
        <UiListActionButton
          onClick={() => window.open(action.docsUrl, "_blank", "noopener,noreferrer")}
          size="sm"
          stopPropagation
          title="查看接入文档"
        >
          <ExternalLink className="h-3 w-3" />
        </UiListActionButton>
      ) : null}
      <UiListActionButton
        className="text-(--primary)"
        onClick={onConfigure}
        size="sm"
        stopPropagation
        title="设置机器人"
        visibility="visible"
      >
        <Settings2 className="h-3 w-3" />
      </UiListActionButton>
    </div>
  );
}

function ChannelCardContent({
  model,
  title,
}: {
  model: ChannelCardModel;
  title: string;
}) {
  return (
    <div className="min-w-0 flex-1">
      <div className="flex min-w-0 items-center gap-2">
        <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
          {title}
        </span>
        {model.badges.map((badge) => (
          <UiBadge key={badge.label} size="xs" tone={badge.tone}>
            {badge.label}
          </UiBadge>
        ))}
      </div>
      <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
        {model.description}
      </div>
      <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-[11px] leading-4 text-(--text-soft)">
        {model.metadata.map((value) => (
          <span className="min-w-0 truncate" key={value}>{value}</span>
        ))}
      </div>
      <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5">
        {model.stats.map((stat) => (
          <ChannelStatPill
            icon={CHANNEL_STAT_ICONS[stat.icon]}
            key={stat.icon}
            label={stat.label}
            tone={stat.tone}
            value={stat.value}
          />
        ))}
      </div>
      {model.runtimeNote ? (
        <div className="mt-0.5 truncate text-[11px] leading-4 text-(--text-soft)">
          {model.runtimeNote}
        </div>
      ) : null}
    </div>
  );
}

export function ChannelCard({
  item,
  onConfigure,
}: {
  item: ChannelConfigView;
  onConfigure: (item: ChannelConfigView) => void;
}) {
  const model = buildChannelCardModel(item);
  const planned = model.action.kind === "planned";
  const configure = () => onConfigure(item);

  return (
    <UiListRow
      className={cn(
        "min-h-[72px] rounded-[14px] px-2 py-1.5",
        planned && "cursor-default opacity-70",
      )}
      leading={<ChannelIcon type={item.channel_type} />}
      onClick={planned ? undefined : configure}
      right={(
        <ChannelCardActions action={model.action} onConfigure={configure} />
      )}
    >
      <ChannelCardContent model={model} title={item.title} />
    </UiListRow>
  );
}
