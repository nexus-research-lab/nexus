import {
  Clock3,
  ExternalLink,
  Settings2,
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
} from "./channel-catalog-model";

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
    <div className="flex shrink-0 items-center gap-1">
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
        <span className="truncate text-[14px] font-medium text-(--text-strong)">
          {title}
        </span>
        {model.badges.map((badge) => (
          <UiBadge key={badge.label} size="xs" tone={badge.tone}>
            {badge.label}
          </UiBadge>
        ))}
      </div>
      <div className="mt-0.5 truncate text-[12px] leading-[1.125rem] text-(--text-muted)">
        {model.description}
      </div>
      <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[10px] leading-4 text-(--text-soft)">
        {model.metadata.map((value, index) => (
          <span className="inline-flex min-w-0 items-center gap-1.5" key={value}>
            {index > 0 ? <span aria-hidden="true">·</span> : null}
            <span className="truncate">{value}</span>
          </span>
        ))}
        {model.stats.map((stat) => (
          <span className="inline-flex items-center gap-1.5" key={stat.kind}>
            <span aria-hidden="true">·</span>
            <span
              className={cn(
                "tabular-nums",
                stat.tone === "warning" ? "text-(--warning)" : "text-(--text-muted)",
              )}
            >
              {stat.label} {stat.value}
            </span>
          </span>
        ))}
      </div>
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
        "min-h-[60px] rounded-[8px] px-2 py-1",
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
