import type {
  ChannelConfigView,
  ImChannelType,
} from "@/lib/api/capability/channel-api";
import type { TranslationKey } from "@/shared/i18n/messages";

import { isChannelPlanned } from "../channel-model";

export type ChannelFilter =
  | "all"
  | "configured"
  | "connected"
  | "planned"
  | "unconfigured";

export const CHANNEL_FILTER_OPTIONS: ReadonlyArray<{
  labelKey: TranslationKey;
  value: ChannelFilter;
}> = [
  { value: "all", labelKey: "capability.channels_filter_all" },
  { value: "connected", labelKey: "capability.channels_filter_connected" },
  { value: "configured", labelKey: "capability.channels_filter_configured" },
  { value: "unconfigured", labelKey: "capability.channels_filter_unconfigured" },
  { value: "planned", labelKey: "capability.channels_filter_planned" },
];

const CHANNEL_ORDER: ImChannelType[] = [
  "dingtalk",
  "wechat",
  "weixin-personal",
  "feishu",
  "telegram",
  "discord",
];

const CHANNEL_FILTERS: Record<
  ChannelFilter,
  (item: ChannelConfigView) => boolean
> = {
  all: () => true,
  configured: (item) => item.configured && !isChannelPlanned(item),
  connected: (item) => item.connection_state === "connected",
  planned: isChannelPlanned,
  unconfigured: (item) => !item.configured && !isChannelPlanned(item),
};

const CHANNEL_DESCRIPTIONS: Array<{
  description: string;
  matches: (item: ChannelConfigView) => boolean;
}> = [
  {
    matches: isChannelPlanned,
    description: "该频道将在后续版本补充，目前仅保留入口和信息结构。",
  },
  {
    matches: (item) => item.runtime_status === "external_adapter" && !item.configured,
    description: "选择处理智能体后，按通道说明完成外部连接。",
  },
  {
    matches: (item) => item.configured,
    description: "消息会进入绑定的处理智能体。",
  },
  {
    matches: () => true,
    description: "选择一个智能体并填写机器人凭证后，即可开始处理来自该渠道的消息。",
  },
];

export function describeChannel(item: ChannelConfigView): string {
  return CHANNEL_DESCRIPTIONS.find(({ matches }) => matches(item))?.description ?? "";
}

export function filterChannels(
  channels: ChannelConfigView[],
  filter: ChannelFilter,
  searchQuery: string,
): ChannelConfigView[] {
  const query = searchQuery.trim().toLowerCase();
  return [...channels]
    .sort((left, right) => channelOrder(left) - channelOrder(right))
    .filter((item) => matchesChannelQuery(item, query) && CHANNEL_FILTERS[filter](item));
}

function channelOrder(item: ChannelConfigView): number {
  const index = CHANNEL_ORDER.indexOf(item.channel_type);
  return index < 0 ? CHANNEL_ORDER.length : index;
}

function matchesChannelQuery(item: ChannelConfigView, query: string): boolean {
  if (!query) {
    return true;
  }
  return [
    item.title,
    item.bot_label,
    item.channel_type,
    item.agent_name ?? "",
  ].some((value) => value.toLowerCase().includes(query));
}
