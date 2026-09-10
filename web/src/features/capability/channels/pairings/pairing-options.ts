// INPUT: 稳定渠道/会话/配对状态协议值和当前翻译器。
// OUTPUT: 单一键表生成的双语标签与选项；默认中文标签仅用于既有搜索兼容。
// POS: 配对标签的唯一所有者，不改变命令载荷中的枚举。
import type { ImChatType, ImChannelType, ImPairingStatus } from "@/lib/api/capability/channel-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type TranslationKey } from "@/shared/i18n/messages";

const CHANNEL_KEYS: Record<ImChannelType, TranslationKey> = {
  dingtalk: "capability.pairing_channel_dingtalk",
  wechat: "capability.pairing_channel_wechat",
  "weixin-personal": "capability.pairing_channel_weixin_personal",
  feishu: "capability.pairing_channel_feishu",
  telegram: "capability.pairing_channel_telegram",
  discord: "capability.pairing_channel_discord",
};
const STATUS_KEYS: Record<ImPairingStatus, TranslationKey> = {
  pending: "capability.pairing_status_pending",
  active: "capability.pairing_status_active",
  disabled: "capability.pairing_status_disabled",
  rejected: "capability.pairing_status_rejected",
};
const CHAT_KEYS: Record<ImChatType, TranslationKey> = {
  dm: "capability.pairing_chat_dm", group: "capability.pairing_chat_group",
};
function labels<Value extends string>(keys: Record<Value, TranslationKey>, t: I18nContextValue["t"]): Record<Value, string> {
  return Object.fromEntries((Object.keys(keys) as Value[]).map((value) => [value, t(keys[value])])) as Record<Value, string>;
}
function options<Value extends string>(values: Record<Value, string>): Array<{ value: Value; label: string }> {
  return (Object.keys(values) as Value[]).map((value) => ({ value, label: values[value] }));
}
export function getPairingLabels(t: I18nContextValue["t"]) {
  return { channels: labels(CHANNEL_KEYS, t), statuses: labels(STATUS_KEYS, t), chatTypes: labels(CHAT_KEYS, t) };
}
export function getPairingOptions(t: I18nContextValue["t"]) {
  const translated = getPairingLabels(t);
  return {
    channels: options(translated.channels),
    chatTypes: options(translated.chatTypes),
    initialStatuses: (["active", "pending", "disabled"] as const).map((value) => ({ value, label: translated.statuses[value] })),
  };
}
const defaultLabels = getPairingLabels((key) => MESSAGES.zh[key]);
export const CHANNEL_LABELS = defaultLabels.channels;
export const CHAT_TYPE_LABELS = defaultLabels.chatTypes;
