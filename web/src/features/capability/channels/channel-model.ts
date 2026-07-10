import type { ChannelConfigView } from "@/lib/api/channel-api";

export interface ChannelFeedback {
  message: string;
  title: string;
  tone: "error" | "success";
}

export function isChannelPlanned(item: ChannelConfigView): boolean {
  return item.runtime_status === "planned";
}
