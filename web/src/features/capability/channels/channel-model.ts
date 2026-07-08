import {
  ChannelConfigView,
  ChannelLoginView,
  ImChannelType,
} from "@/lib/api/channel-api";

export function isChannelPlanned(item: ChannelConfigView) {
  return item.runtime_status === "planned";
}

export function isPersonalWeixinChannel(channelType: ImChannelType) {
  return channelType === "weixin-personal";
}

export function isChannelLoginRunning(view: ChannelLoginView | null) {
  return view?.status === "running";
}
