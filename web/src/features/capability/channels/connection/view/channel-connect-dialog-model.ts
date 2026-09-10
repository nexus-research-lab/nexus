import type { I18nContextValue } from "@/shared/i18n/i18n-context";

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

export function getChannelSubmitLabel(state: ChannelSubmitState, t: I18nContextValue["t"]): string {
  const candidates = [
    [state.planned, t("capability.channel_not_available")],
    [state.saving, t("common.saving")],
    [state.loginLoading, t("capability.channel_qr_starting")],
    [state.loginRunning, t("capability.channel_qr_waiting")],
    [state.supportsQRCode, t("capability.channel_qr_start")],
    [true, t("capability.channel_connect")],
  ] as const;
  return candidates.find(([matches]) => matches)?.[1] ?? t("capability.channel_connect");
}

export function getChannelDeleteDialogCopy(
  target: PendingChannelDelete | null,
  item: ChannelConfigView,
  t: I18nContextValue["t"],
): ChannelDeleteDialogCopy {
  if (!target) {
    return { confirmText: t("capability.channel_delete_account"), message: "", title: t("capability.channel_delete_weixin_account") };
  }
  if (target.kind === "channel") {
    return {
      confirmText: t("capability.channel_disconnect"),
      message: t("capability.channel_disconnect_confirm").replace("{channel}", item.title),
      title: t("capability.channel_disconnect"),
    };
  }
  return {
    confirmText: t("capability.channel_delete_account"),
    message: t("capability.channel_delete_account_confirm").replace("{account}", target.account.user_id || target.account.account_id),
    title: t("capability.channel_delete_weixin_account"),
  };
}
