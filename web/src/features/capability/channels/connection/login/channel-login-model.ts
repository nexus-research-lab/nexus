// INPUT: Server Channel login snapshot plus the current translation function.
// OUTPUT: Safe status, identity, QR and terminal recovery copy for the login panel.
// POS: Login presentation boundary; raw provider output, errors and login IDs stay hidden.
import type { ChannelLoginView } from "@/lib/api/capability/channel-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
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
      failure: {
        impact: string;
        nextStep: string;
        title: string;
        tone: "error" | "warning";
      } | null;
      identity: string;
      kind: "session";
      progress: string;
      qrPayload: string;
      qrRequired: boolean;
      status: ChannelLoginStatusPresentation;
      verifyCodeHint: string;
    };

type ChannelLoginStatusDefinition = Omit<
  ChannelLoginStatusPresentation,
  "label"
> & { labelKey: TranslationKey };

const LOGIN_STATUS_PRESENTATIONS: Record<string, ChannelLoginStatusDefinition> = {
  cancelled: {
    icon: "terminal",
    labelKey: "capability.channel_login_status_cancelled",
    tone: "warning",
  },
  error: {
    icon: "warning",
    labelKey: "capability.channel_login_status_error",
    tone: "danger",
  },
  expired: {
    icon: "warning",
    labelKey: "capability.channel_login_status_expired",
    tone: "warning",
  },
  running: {
    icon: "terminal",
    labelKey: "capability.channel_login_status_running",
    tone: "info",
  },
  succeeded: {
    icon: "success",
    labelKey: "capability.channel_login_status_succeeded",
    tone: "success",
  },
  verify_code_required: {
    icon: "terminal",
    labelKey: "capability.channel_login_status_verify_required",
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

function loginStatusDefinition(
  status: string,
): ChannelLoginStatusDefinition | undefined {
  return Object.prototype.hasOwnProperty.call(LOGIN_STATUS_PRESENTATIONS, status)
    ? LOGIN_STATUS_PRESENTATIONS[status]
    : undefined;
}

export function isChannelLoginRunning(view: ChannelLoginView | null): boolean {
  return view?.status === "running";
}

function resolveLoginStatus(
  status: string,
  t: I18nContextValue["t"],
): ChannelLoginStatusPresentation {
  const presentation = loginStatusDefinition(status);
  if (!presentation) {
    return {
      ...DEFAULT_LOGIN_STATUS_PRESENTATION,
      label: t("capability.channel_login_status_pending"),
    };
  }
  return {
    icon: presentation.icon,
    label: t(presentation.labelKey),
    tone: presentation.tone,
  };
}

function resolveLoginIdentity(
  view: ChannelLoginView,
  t: I18nContextValue["t"],
): string {
  return [view.user_id, view.account_id].find(Boolean)
    ?? t("capability.channel_login_session_label");
}

function resolveLoginProgress(
  view: ChannelLoginView,
  t: I18nContextValue["t"],
): string {
  switch (view.status) {
    case "running":
      return t("capability.channel_login_running_message");
    case "verify_code_required":
      return t("capability.channel_login_verify_required_message");
    case "succeeded":
      return t("capability.channel_login_succeeded_message");
    case "expired":
      return t("capability.channel_login_expired_message");
    case "cancelled":
      return t("capability.channel_login_cancelled_message");
    case "error":
      return t("capability.channel_login_failed_message");
    default:
      return t("capability.channel_login_status_pending_message");
  }
}

function resolveTerminalFailure(
  view: ChannelLoginView,
  t: I18nContextValue["t"],
): Extract<ChannelLoginPanelModel, { kind: "session" }>["failure"] {
  if (!loginStatusDefinition(view.status)) {
    return {
      impact: t("capability.channel_login_unknown_status_impact"),
      nextStep: t("capability.channel_login_unknown_status_next_step"),
      title: t("capability.channel_login_unknown_status_title"),
      tone: "warning",
    };
  }
  if (view.status === "error") {
    return {
      impact: t("capability.channel_login_failed_impact"),
      nextStep: t("capability.channel_login_failed_next_step"),
      title: t("capability.channel_login_failed_title"),
      tone: "error",
    };
  }
  if (view.status === "expired" || view.status === "cancelled") {
    const expired = view.status === "expired";
    return {
      impact: t(expired
        ? "capability.channel_login_expired_impact"
        : "capability.channel_login_cancelled_impact"),
      nextStep: t(expired
        ? "capability.channel_login_expired_next_step"
        : "capability.channel_login_cancelled_next_step"),
      title: t(expired
        ? "capability.channel_login_expired_title"
        : "capability.channel_login_cancelled_title"),
      tone: "warning",
    };
  }
  return null;
}

export function buildChannelLoginPanelModel(
  view: ChannelLoginView | null,
  t: I18nContextValue["t"],
): ChannelLoginPanelModel {
  if (!view) {
    return { kind: "idle" };
  }
  const failure = resolveTerminalFailure(view, t);

  return {
    failure,
    identity: resolveLoginIdentity(view, t),
    kind: "session",
    progress: failure ? "" : resolveLoginProgress(view, t),
    qrPayload: view.qr_payload ?? "",
    qrRequired: view.status === "running",
    status: resolveLoginStatus(view.status, t),
    verifyCodeHint: view.status === "verify_code_required"
      ? t("capability.channel_login_verify_code_hint")
      : "",
  };
}
