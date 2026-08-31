import type { ChannelConfigView } from "@/lib/api/capability/channel-api";

import type { PendingChannelDelete } from "../channel-connection-model";

export interface ChannelSubmitState {
  loginLoading: boolean;
  loginRunning: boolean;
  planned: boolean;
  saving: boolean;
  supportsQRCode: boolean;
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
    [state.supportsQRCode, "拉起二维码"],
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
      message: `确认断开 ${item.title} 吗？这会删除该频道的配置、已连接账号和配对，并停止消息收发；其他频道不受影响。`,
      title: "断开频道",
    };
  }
  return {
    confirmText: "删除账号",
    message: `确认删除微信账号 ${target.account.user_id || target.account.account_id} 吗？该账号及使用它的配对会被删除并停止消息收发；其他账号不受影响。`,
    title: "删除微信账号",
  };
}
