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

export type ChannelCardStatIcon = "group" | "pending" | "user";

export interface ChannelCardModel {
  action:
    | { kind: "configure"; docsUrl: string }
    | { kind: "planned" };
  badges: ReadonlyArray<{
    label: string;
    tone: "warning";
  }>;
  description: string;
  metadata: string[];
  runtimeNote: string;
  stats: ReadonlyArray<{
    icon: ChannelCardStatIcon;
    label: string;
    tone: "default" | "warning";
    value: number;
  }>;
}

type ChannelCardStatCandidate = ChannelCardModel["stats"][number] & {
  visible: boolean;
};

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

function describeChannel(item: ChannelConfigView): string {
  return CHANNEL_DESCRIPTIONS.find(({ matches }) => matches(item))?.description ?? "";
}

export function buildChannelCardModel(item: ChannelConfigView): ChannelCardModel {
  const planned = isChannelPlanned(item);
  const handlerLabel = item.configured ? item.agent_name || "已绑定" : "未绑定";
  const metadata = [
    { visible: true, value: `机器人：${item.bot_label}` },
    { visible: true, value: `处理：${handlerLabel}` },
    { visible: !item.supports_group, value: "仅私聊" },
  ].filter(({ visible }) => visible).map(({ value }) => value);
  const statCandidates: ChannelCardStatCandidate[] = [
    {
      icon: "user",
      label: "用户",
      tone: "default",
      value: item.stats.paired_user_count,
      visible: true,
    },
    {
      icon: "group",
      label: "群聊",
      tone: "default",
      value: item.stats.paired_group_count,
      visible: item.supports_group,
    },
    {
      icon: "pending",
      label: "待审",
      tone: item.stats.pending_count > 0 ? "warning" : "default",
      value: item.stats.pending_count,
      visible: true,
    },
  ];
  const stats = statCandidates
    .filter(({ visible }) => visible)
    .map(({ visible: _visible, ...stat }) => stat);

  return {
    action: planned
      ? { kind: "planned" }
      : { kind: "configure", docsUrl: item.docs_url ?? "" },
    badges: item.runtime_status === "external_adapter"
      ? [{ label: "外部适配器", tone: "warning" }]
      : [],
    description: describeChannel(item),
    metadata,
    runtimeNote: item.runtime_note ?? "",
    stats,
  };
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
