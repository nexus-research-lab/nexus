// INPUT: Channel 类型与目录/弹窗尺寸语义。
// OUTPUT: 使用准确平台轮廓和 Capability 公共单色容器的频道身份图标。
// POS: Channel 类型到品牌资源的唯一映射；不拥有颜色、边框、圆角或尺寸 recipe。

import { CapabilityBrandIcon } from "@/features/capability/shared/capability-brand-icon";
import { ImChannelType } from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

const CHANNEL_ICONS: Record<
  ImChannelType,
  { src: string; titleKey: TranslationKey }
> = {
  dingtalk: {
    src: "/icon/connector/dingtalk.svg",
    titleKey: "capability.pairing_channel_dingtalk",
  },
  wechat: {
    src: "/icon/channel/wecom.svg",
    titleKey: "capability.pairing_channel_wechat",
  },
  "weixin-personal": {
    src: "/icon/channel/wechat.svg",
    titleKey: "capability.pairing_channel_weixin_personal",
  },
  feishu: {
    src: "/icon/connector/feishu.svg",
    titleKey: "capability.pairing_channel_feishu",
  },
  telegram: {
    src: "/icon/channel/telegram.svg",
    titleKey: "capability.pairing_channel_telegram",
  },
  discord: {
    src: "/icon/channel/discord.svg",
    titleKey: "capability.pairing_channel_discord",
  },
};

export function ChannelIcon({
  type,
  size = "card",
}: {
  type: ImChannelType;
  size?: "card" | "dialog";
}) {
  const { t } = useI18n();
  const icon = CHANNEL_ICONS[type];
  return (
    <CapabilityBrandIcon
      size={size === "dialog" ? "lg" : "md"}
      src={icon.src}
      title={t(icon.titleKey)}
    />
  );
}
