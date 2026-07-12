import type { ChannelLoginView } from "@/lib/api/capability/channel-api";
import type { UiBadgeTone } from "@/shared/ui/display/badge-styles";

export type ChannelLoginStatusIcon = "success" | "terminal" | "warning";

interface ChannelLoginStatusPresentation {
  icon: ChannelLoginStatusIcon;
  label: string;
  tone: UiBadgeTone;
}

export type ChannelLoginPanelModel =
  | { kind: "idle" }
  | {
      error: string;
      identity: string;
      kind: "session";
      output: string;
      qrPayload: string;
      status: ChannelLoginStatusPresentation;
      verifyCodeHint: string;
    };

const LOGIN_STATUS_PRESENTATIONS: Record<string, ChannelLoginStatusPresentation> = {
  cancelled: { icon: "terminal", label: "已取消", tone: "warning" },
  error: { icon: "warning", label: "登录失败", tone: "danger" },
  expired: { icon: "warning", label: "已超时", tone: "warning" },
  running: { icon: "terminal", label: "等待扫码", tone: "info" },
  succeeded: { icon: "success", label: "登录完成", tone: "success" },
  verify_code_required: {
    icon: "terminal",
    label: "需要验证码",
    tone: "warning",
  },
};

const DEFAULT_LOGIN_STATUS_PRESENTATION: Omit<
  ChannelLoginStatusPresentation,
  "label"
> = {
  icon: "terminal",
  tone: "default",
};

export function isChannelLoginRunning(view: ChannelLoginView | null): boolean {
  return view?.status === "running";
}

function resolveLoginStatus(status: string): ChannelLoginStatusPresentation {
  return LOGIN_STATUS_PRESENTATIONS[status] ?? {
    ...DEFAULT_LOGIN_STATUS_PRESENTATION,
    label: status || "未启动",
  };
}

function resolveLoginIdentity(view: ChannelLoginView): string {
  return [view.account_id, view.command].find(Boolean)
    ?? "Nexus iLink QR login";
}

function resolveLoginOutput(view: ChannelLoginView): string {
  const output = view.output?.trimEnd() ?? "";
  return output || (isChannelLoginRunning(view) ? "等待 iLink 扫码状态..." : "");
}

export function buildChannelLoginPanelModel(
  view: ChannelLoginView | null,
): ChannelLoginPanelModel {
  if (!view) {
    return { kind: "idle" };
  }

  return {
    error: view.error ?? "",
    identity: resolveLoginIdentity(view),
    kind: "session",
    output: resolveLoginOutput(view),
    qrPayload: view.qr_payload ?? "",
    status: resolveLoginStatus(view.status),
    verifyCodeHint: view.status === "verify_code_required"
      ? view.verify_code_hint || "输入手机微信显示的数字"
      : "",
  };
}
