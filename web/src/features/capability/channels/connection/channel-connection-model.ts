import type {
  ChannelAccountView,
  ChannelConfigView,
  ChannelCredentialField,
  ImChannelType,
} from "@/lib/api/capability/channel-api";

export interface ChannelConnectionDraft {
  agentId: string;
  config: Record<string, string>;
  credentials: Record<string, string>;
}

export type ChannelPendingAction =
  | { kind: "delete-account"; accountId: string }
  | { kind: "delete-channel" }
  | { kind: "save" }
  | { kind: "verify-code" };

export type PendingChannelDelete =
  | { kind: "account"; account: ChannelAccountView }
  | { kind: "channel" };

const ACCOUNT_STATUS_LABELS: Record<string, string> = {
  configured: "已配置",
  connected: "已连接",
  disabled: "已停用",
  error: "异常",
  pending: "待确认",
};

export function createChannelDraft(
  item: ChannelConfigView,
  fallbackAgentId: string,
): ChannelConnectionDraft {
  return {
    agentId: item.agent_id || fallbackAgentId,
    config: item.public_config || {},
    credentials: {},
  };
}

export function isPersonalWeixinChannel(channelType: ImChannelType): boolean {
  return channelType === "weixin-personal";
}

export function channelAccountStatusLabel(status: string): string {
  return (ACCOUNT_STATUS_LABELS[status] ?? status) || "未知";
}

export function buildDiscordOauthUrl(config: Record<string, string>): string {
  const appId = config.application_id?.trim();
  if (!appId) {
    return "";
  }
  const params = new URLSearchParams({
    client_id: appId,
    permissions: "274877975552",
    scope: "bot applications.commands",
  });
  return `https://discord.com/oauth2/authorize?${params.toString()}`;
}

export function channelFieldAutocomplete(field: ChannelCredentialField): string {
  return field.secret ? "new-password" : "off";
}

export function channelFieldInputName(
  channelType: ImChannelType,
  index: number,
): string {
  return `nexus-im-channel-${channelType}-field-${index}`;
}
