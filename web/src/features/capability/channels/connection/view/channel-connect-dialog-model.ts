import type { ChannelConfigView } from "@/lib/api/capability/channel-api";

import type { PendingChannelDelete } from "../channel-connection-model";

export interface ChannelSubmitState {
  loginLoading: boolean;
  loginRunning: boolean;
  planned: boolean;
  saving: boolean;
  supportsPersonalWeixinLogin: boolean;
}

export interface ChannelDeleteDialogCopy {
  confirmText: string;
  message: string;
  title: string;
}

const EMPTY_ACCOUNT_DELETE_COPY: ChannelDeleteDialogCopy = {
  confirmText: "删除账号",
  message: "",
  title: "删除微信账号",
};

export function getChannelSubmitLabel(state: ChannelSubmitState): string {
  const candidates = [
    [state.planned, "未上线"],
    [state.saving, "保存中..."],
    [state.loginLoading, "拉起二维码..."],
    [state.loginRunning, "等待扫码..."],
    [state.supportsPersonalWeixinLogin, "拉起二维码"],
    [true, "连接"],
  ] as const;
  return candidates.find(([matches]) => matches)?.[1] ?? "连接";
}

export function getChannelDeleteDialogCopy(
  target: PendingChannelDelete | null,
  item: ChannelConfigView,
): ChannelDeleteDialogCopy {
  if (!target) {
    return EMPTY_ACCOUNT_DELETE_COPY;
  }
  if (target.kind === "channel") {
    return {
      confirmText: "断开频道",
      message: `确认断开 ${item.title} 吗？这会停止该频道的机器人连接，但不会删除已有配对。`,
      title: "断开频道",
    };
  }
  return {
    confirmText: "删除账号",
    message: `确认删除微信账号 ${target.account.user_id || target.account.account_id} 吗？已有配对不会删除，但该账号会停止接收和回投消息。`,
    title: "删除微信账号",
  };
}
