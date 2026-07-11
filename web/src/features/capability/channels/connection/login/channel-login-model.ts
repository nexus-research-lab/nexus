import type { ChannelLoginView } from "@/lib/api/capability/channel-api";
import type { UiBadgeTone } from "@/shared/ui/display/badge-styles";

const LOGIN_STATUS_LABELS: Record<string, string> = {
  cancelled: "已取消",
  error: "登录失败",
  expired: "已超时",
  running: "等待扫码",
  succeeded: "登录完成",
  verify_code_required: "需要验证码",
};

const LOGIN_STATUS_TONES: Record<string, UiBadgeTone> = {
  cancelled: "warning",
  error: "danger",
  expired: "warning",
  running: "info",
  succeeded: "success",
  verify_code_required: "warning",
};

export function isChannelLoginRunning(view: ChannelLoginView | null): boolean {
  return view?.status === "running";
}

export function channelLoginStatusLabel(status: string): string {
  return (LOGIN_STATUS_LABELS[status] ?? status) || "未启动";
}

export function channelLoginStatusTone(status: string): UiBadgeTone {
  return LOGIN_STATUS_TONES[status] ?? "default";
}
