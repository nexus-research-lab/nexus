import {
  ChannelConfigView,
  ChannelLoginView,
  ImChannelType,
} from "@/lib/api/channel-api";

export function is_channel_planned(item: ChannelConfigView) {
  return item.runtime_status === "planned";
}

export function is_personal_weixin_channel(channel_type: ImChannelType) {
  return channel_type === "weixin-personal";
}

export function is_channel_login_running(view: ChannelLoginView | null) {
  return view?.status === "running";
}
