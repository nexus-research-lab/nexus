/**
 * INPUT: 已投影的频道身份、状态、使用元数据与配置动作。
 * OUTPUT: 显示平台用途、状态和使用事实的频道条目。
 * POS: 频道目录卡片纯视图。
 */
import {
  Clock3,
  ExternalLink,
  Settings2,
} from "lucide-react";

import { CAPABILITY_DIRECTORY_ROW_CLASS_NAME } from "@/features/capability/shared/capability-page-layout";
import { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { UiLinkButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  const { t } = useI18n();

  if (action.kind === "planned") {
    return (
      <span aria-hidden="true" className="flex h-8 w-8 items-center justify-center text-(--icon-muted)">
        <Clock3 className="h-3.5 w-3.5" />
      </span>
    );
  }

  return (
    <div className="flex shrink-0 items-center gap-1">
      {action.docsUrl ? (
        <UiLinkButton
          aria-label={t("capability.channel_docs_action")}
          className="h-7 w-7 p-0"
          href={action.docsUrl}
          onClick={(event) => event.stopPropagation()}
          rel="noopener noreferrer"
          size="xs"
          target="_blank"
          title={t("capability.channel_docs_action")}
          variant="text"
        >
          <ExternalLink className="h-3 w-3" />
        </UiLinkButton>
      ) : null}
      <UiListActionButton
        aria-label={t("capability.channel_configure_action")}
        className="text-(--primary)"
        onClick={onConfigure}
        size="sm"
        stopPropagation
        title={t("capability.channel_configure_action")}
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
  const { t } = useI18n();
  return (
    <div className="min-w-0 flex-1">
      <div className="flex min-w-0 items-center gap-2">
        <span className={cn(
          "truncate",
          getUiTypographyClassName({ role: "control", tone: "strong", weight: "medium" }),
        )}>
          {title}
        </span>
        {model.badges.map((badge) => (
          <UiBadge key={badge.labelKey} size="xs" tone={badge.tone}>
            {t(badge.labelKey)}
          </UiBadge>
        ))}
      </div>
      <div className={cn(
        "mt-0.5 truncate",
        getUiTypographyClassName({ role: "metadata", tone: "muted" }),
      )}>
        {model.description}
      </div>
      <div className={cn(
        "mt-0.5 flex min-w-0 flex-nowrap items-center gap-x-2.5 overflow-hidden",
        getUiTypographyClassName({ role: "caption", tone: "soft" }),
      )}>
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
                getUiTypographyClassName({
                  role: "caption",
                  tone: stat.tone === "warning" ? "warning" : "muted",
                }),
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
        CAPABILITY_DIRECTORY_ROW_CLASS_NAME,
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
