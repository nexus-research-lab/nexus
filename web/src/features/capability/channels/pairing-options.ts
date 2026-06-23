import { ImChatType, ImChannelType, ImPairingStatus } from "@/lib/api/channel-api";

export const CHANNEL_LABELS: Record<ImChannelType, string> = {
  dingtalk: "钉钉",
  wechat: "企业微信",
  "weixin-personal": "微信",
  feishu: "飞书",
  telegram: "Telegram",
  discord: "Discord",
};

export const STATUS_LABELS: Record<ImPairingStatus, string> = {
  pending: "待处理",
  active: "已授权",
  disabled: "已停用",
  rejected: "已拒绝",
};

export const CHANNEL_OPTIONS = (Object.entries(CHANNEL_LABELS) as Array<[ImChannelType, string]>).map(([value, label]) => ({
  value,
  label,
}));

export const CHAT_TYPE_OPTIONS: Array<{ value: ImChatType; label: string }> = [
  { value: "dm", label: "用户" },
  { value: "group", label: "群聊" },
];

export const CREATE_PAIRING_STATUS_OPTIONS: Array<{ value: ImPairingStatus; label: string }> = [
  { value: "active", label: STATUS_LABELS.active },
  { value: "pending", label: STATUS_LABELS.pending },
  { value: "disabled", label: STATUS_LABELS.disabled },
];
