// INPUT: Channel resource views and domain-confirmed user feedback facts.
// OUTPUT: Shared Channel catalog predicates and strict feedback contracts.
// POS: Channel Web domain model; it never derives mutation outcomes from copy.
import type { ChannelConfigView } from "@/lib/api/capability/channel-api";

interface ChannelFeedbackBase {
  action?: {
    label: string;
    onClick: () => void;
  };
  dismissible?: boolean;
  message: string;
  title: string;
}

export type ChannelFeedback =
  | ChannelFeedbackBase & {
    impact: string;
    nextStep: string;
    tone: "error" | "warning";
  }
  | ChannelFeedbackBase & {
    tone: "success";
  };

export function isChannelPlanned(item: ChannelConfigView): boolean {
  return item.runtime_status === "planned";
}
