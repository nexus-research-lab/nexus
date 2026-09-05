// INPUT: Safe Channel login projection, controller recovery notice, and verification action.
// OUTPUT: QR/status UI with complete result-impact-next-step failure surfaces.
// POS: Channel login view; it never renders raw provider output, errors, or login IDs.
"use client";

import { useState } from "react";
import {
  CircleCheck,
  QrCode,
  Terminal,
  TriangleAlert,
} from "lucide-react";

import type {
  ChannelLoginView,
  ImChannelType,
} from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { FeedbackBanner } from "@/shared/ui/feedback/feedback-banner";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { UiInput } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import {
  buildChannelLoginPanelModel,
  type ChannelLoginPanelModel,
  type ChannelLoginStatusIcon,
} from "./channel-login-model";
import { LoginQRCode } from "./login-qr-code";

const LOGIN_STATUS_ICONS: Record<ChannelLoginStatusIcon, typeof Terminal> = {
  success: CircleCheck,
  terminal: Terminal,
  warning: TriangleAlert,
};

function channelLoginDescription(
  channelType: ImChannelType,
  channelTitle: string,
): string {
  if (channelType === "feishu") {
    return "不填写下方凭据时，保存后打开飞书官方扫码页；可选择已有应用并补齐权限，也可创建新应用。填写已有 App ID / Secret 则直接连接。";
  }
  if (channelType === "weixin-personal") {
    return "Nexus 会先保存当前配置，再通过微信官方接口生成登录二维码。";
  }
  return `不填写下方凭据时，Nexus 会通过 ${channelTitle} 官方接口生成二维码并自动保存凭据；也可填写已有凭据直接连接。`;
}

function ChannelLoginHeader({
  channelTitle,
  channelType,
}: {
  channelTitle: string;
  channelType: ImChannelType;
}) {
  return (
    <div className="min-w-0">
      <h3 className={cn(
        "flex items-center gap-2",
        getUiTypographyClassName({
          role: "control",
          tone: "strong",
          weight: "semibold",
        }),
      )}>
        <QrCode className="h-4 w-4 text-(--primary)" />
        扫码连接
      </h3>
      <p className={cn(
        "mt-1",
        getUiTypographyClassName({ role: "metadata", tone: "muted" }),
      )}>
        {channelLoginDescription(channelType, channelTitle)}
      </p>
    </div>
  );
}

function ChannelLoginVerifyCode({
  hint,
  loading,
  blocked,
  onSubmit,
}: {
  hint: string;
  loading: boolean;
  blocked: boolean;
  onSubmit: (value: string) => Promise<boolean>;
}) {
  const [verifyCode, setVerifyCode] = useState("");
  const submit = async () => {
    if (await onSubmit(verifyCode)) {
      setVerifyCode("");
    }
  };

  return (
    <UiPanel
      className="border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)]"
      padding="sm"
      radius="sm"
    >
      <div className={cn(
        "mb-2",
        getUiTypographyClassName({
          role: "metadata",
          tone: "strong",
          weight: "semibold",
        }),
      )}>
        {hint}
      </div>
      <div className="flex gap-2">
        <UiInput
          onChange={(event) => setVerifyCode(event.target.value)}
          placeholder="验证码"
          value={verifyCode}
          variant="dialog"
        />
        <UiButton
          aria-busy={loading || undefined}
          disabled={!verifyCode.trim() || loading || blocked}
          onClick={() => void submit()}
          size="sm"
          tone="primary"
          type="button"
          variant="solid"
        >
          提交
        </UiButton>
      </div>
    </UiPanel>
  );
}

function ChannelLoginSession({
  loading,
  mutationBlocked,
  model,
  onSubmitVerifyCode,
}: {
  loading: boolean;
  mutationBlocked: boolean;
  model: Extract<ChannelLoginPanelModel, { kind: "session" }>;
  onSubmitVerifyCode: (value: string) => Promise<boolean>;
}) {
  const StatusIcon = LOGIN_STATUS_ICONS[model.status.icon];
  return (
    <div className="mt-3 space-y-3">
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <UiBadge size="xs" tone={model.status.tone}>
          <StatusIcon className="mr-1 h-3 w-3" />
          {model.status.label}
        </UiBadge>
        <code className={cn(
          "surface-radius-sm min-w-0 truncate border border-(--divider-subtle-color) px-2 py-1",
          getUiTypographyClassName({ role: "code", tone: "muted" }),
        )}>
          {model.identity}
        </code>
      </div>
      <LoginQRCode payload={model.qrPayload} required={model.qrRequired} />
      {model.verifyCodeHint ? (
        <ChannelLoginVerifyCode
          blocked={mutationBlocked}
          hint={model.verifyCodeHint}
          loading={loading}
          onSubmit={onSubmitVerifyCode}
        />
      ) : null}
      {model.progress ? (
        <UiPanel
          className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}
          padding="sm"
          radius="sm"
        >
          {model.progress}
        </UiPanel>
      ) : null}
      {model.failure ? (
        <FeedbackBanner {...model.failure} />
      ) : null}
    </div>
  );
}

export function ChannelLoginPanel({
  channelTitle,
  channelType,
  loading,
  loginView,
  mutationBlocked,
  onSubmitVerifyCode,
  recoveryNotice,
}: {
  channelTitle: string;
  channelType: ImChannelType;
  loading: boolean;
  loginView: ChannelLoginView | null;
  mutationBlocked: boolean;
  onSubmitVerifyCode: (value: string) => Promise<boolean>;
  recoveryNotice: FeedbackBannerProps | null;
}) {
  const { t } = useI18n();
  const model = buildChannelLoginPanelModel(loginView, t);

  return (
    <UiPanel padding="sm" radius="sm">
      <ChannelLoginHeader channelTitle={channelTitle} channelType={channelType} />
      {recoveryNotice ? (
        <div className="mt-3">
          <FeedbackBanner {...recoveryNotice} />
        </div>
      ) : null}
      {model.kind === "session" ? (
        <ChannelLoginSession
          loading={loading}
          mutationBlocked={mutationBlocked}
          model={model}
          onSubmitVerifyCode={onSubmitVerifyCode}
        />
      ) : null}
    </UiPanel>
  );
}
