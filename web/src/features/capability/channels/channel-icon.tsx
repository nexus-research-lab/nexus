import {
  Gamepad2,
  MessageCircle,
  QrCode,
  Send,
} from "lucide-react";

import { ImChannelType } from "@/lib/api/capability/channel-api";
import { cn } from "@/shared/ui/class-name";

const CHANNEL_STYLES: Record<ImChannelType, { icon: typeof Send; cn_name: string }> = {
  dingtalk: { icon: Send, cn_name: "bg-[#1677ff] text-white" },
  wechat: { icon: MessageCircle, cn_name: "bg-[#15c45d] text-white" },
  "weixin-personal": { icon: QrCode, cn_name: "bg-[#10a36a] text-white" },
  feishu: { icon: Send, cn_name: "bg-[#356bff] text-white" },
  telegram: { icon: Send, cn_name: "bg-[#28a8ea] text-white" },
  discord: { icon: Gamepad2, cn_name: "bg-[#5865f2] text-white" },
};

export function ChannelIcon({
  type,
  size = "card",
}: {
  type: ImChannelType;
  size?: "card" | "dialog";
}) {
  const style = CHANNEL_STYLES[type];
  const Icon = style.icon;
  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center border border-white/35",
        size === "dialog" && "shadow-(--surface-avatar-shadow)",
        size === "dialog" ? "h-[52px] w-[52px] rounded-[18px]" : "h-8 w-8 rounded-[7px]",
        style.cn_name,
      )}
    >
      <Icon className={size === "dialog" ? "h-[26px] w-[26px]" : "h-3.5 w-3.5"} />
    </span>
  );
}
