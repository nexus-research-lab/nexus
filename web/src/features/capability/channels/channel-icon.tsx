// INPUT: Channel 类型与目录/弹窗尺寸语义。
// OUTPUT: 使用准确平台轮廓和 Capability 公共单色容器的频道身份图标。
// POS: Channel 类型到品牌资源的唯一映射；不拥有颜色、边框、圆角或尺寸 recipe。

import { CapabilityBrandIcon } from "@/features/capability/shared/capability-brand-icon";
import { ImChannelType } from "@/lib/api/capability/channel-api";

const CHANNEL_ICONS: Record<ImChannelType, { src: string; title: string }> = {
  dingtalk: { src: "/icon/connector/dingtalk.svg", title: "钉钉" },
  wechat: { src: "/icon/channel/wecom.svg", title: "企业微信" },
  "weixin-personal": { src: "/icon/channel/wechat.svg", title: "微信" },
  feishu: { src: "/icon/connector/feishu.svg", title: "飞书" },
  telegram: { src: "/icon/channel/telegram.svg", title: "Telegram" },
  discord: { src: "/icon/channel/discord.svg", title: "Discord" },
};

export function ChannelIcon({
  type,
  size = "card",
}: {
  type: ImChannelType;
  size?: "card" | "dialog";
}) {
  const icon = CHANNEL_ICONS[type];
  return (
    <CapabilityBrandIcon
      size={size === "dialog" ? "lg" : "md"}
      src={icon.src}
      title={icon.title}
    />
  );
}
