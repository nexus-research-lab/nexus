/**
 * INPUT: Exact Channel configuration/account mutation intent and a later Channel catalog read.
 * OUTPUT: Observable completion evidence without replaying the mutation.
 * POS: Channel connection reconciliation model; secret rotations stay unproven without a receipt.
 */
import type {
  ChannelConfigView,
  ImChannelType,
} from "@/lib/api/capability/channel-api";

export type ChannelConnectionIntent =
  | {
      agentId: string;
      baseHadCredentials: boolean;
      channelType: ImChannelType;
      kind: "save";
      publicConfig: Record<string, string>;
      wroteSecrets: boolean;
    }
  | { channelType: ImChannelType; kind: "delete-channel" }
  | {
      accountId: string;
      channelType: ImChannelType;
      kind: "delete-account";
    };

export type ChannelConnectionReconcileOutcome = "applied" | "unproven";

export function reconcileChannelConnectionIntent(
  intent: ChannelConnectionIntent,
  items: ChannelConfigView[],
): ChannelConnectionReconcileOutcome {
  const current = items.find(
    (item) => item.channel_type === intent.channelType,
  );
  if (!current) {
    return "unproven";
  }
  if (intent.kind === "delete-channel") {
    return current.configured ? "unproven" : "applied";
  }
  if (intent.kind === "delete-account") {
    return (current.accounts ?? []).some(
      (account) => account.account_id === intent.accountId,
    )
      ? "unproven"
      : "applied";
  }
  return channelSaveReached(intent, current) ? "applied" : "unproven";
}

function channelSaveReached(
  intent: Extract<ChannelConnectionIntent, { kind: "save" }>,
  current: ChannelConfigView,
): boolean {
  if (!current.configured || current.agent_id !== intent.agentId) {
    return false;
  }
  const publicConfig = current.public_config ?? {};
  if (Object.entries(intent.publicConfig).some(
    ([key, value]) => publicConfig[key] !== value,
  )) {
    return false;
  }
  if (!intent.wroteSecrets) {
    return true;
  }
  // has_credentials proves an initial credential write, but cannot prove that
  // an already-present secret was rotated to this request's opaque value.
  return !intent.baseHadCredentials && current.has_credentials;
}
