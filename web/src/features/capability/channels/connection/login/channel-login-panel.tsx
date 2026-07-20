"use client";

import { useState } from "react";
import {
  CircleCheck,
  QrCode,
  Terminal,
  TriangleAlert,
} from "lucide-react";

import type { ChannelLoginView } from "@/lib/api/capability/channel-api";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiButton } from "@/shared/ui/button/button";
import { UiInput } from "@/shared/ui/form/form-control";
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

function ChannelLoginHeader() {
  return (
    <div className="min-w-0">
      <div className="flex items-center gap-2 text-[13px] font-semibold text-(--text-strong)">
        <QrCode className="h-4 w-4 text-(--primary)" />
        扫码登录
      </div>
      <p className="mt-1 text-[12px] leading-5 text-(--text-muted)">
        Nexus 会先保存当前配置，再请求腾讯 iLink Bot API 生成二维码。
      </p>
    </div>
  );
}

function ChannelLoginVerifyCode({
  hint,
  loading,
  onSubmit,
}: {
  hint: string;
  loading: boolean;
  onSubmit: (value: string) => void;
}) {
  const [verifyCode, setVerifyCode] = useState("");
  const submit = () => {
    onSubmit(verifyCode);
    setVerifyCode("");
  };

  return (
    <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-3 py-3">
      <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">
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
          disabled={!verifyCode.trim() || loading}
          onClick={submit}
          size="sm"
          tone="primary"
          type="button"
          variant="solid"
        >
          提交
        </UiButton>
      </div>
    </div>
  );
}

function ChannelLoginSession({
  loading,
  model,
  onSubmitVerifyCode,
}: {
  loading: boolean;
  model: Extract<ChannelLoginPanelModel, { kind: "session" }>;
  onSubmitVerifyCode: (value: string) => void;
}) {
  const StatusIcon = LOGIN_STATUS_ICONS[model.status.icon];
  return (
    <div className="mt-3 space-y-3">
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <UiBadge size="xs" tone={model.status.tone}>
          <StatusIcon className="mr-1 h-3 w-3" />
          {model.status.label}
        </UiBadge>
        <code className="min-w-0 truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
          {model.identity}
        </code>
      </div>
      <LoginQRCode payload={model.qrPayload} />
      {model.verifyCodeHint ? (
        <ChannelLoginVerifyCode
          hint={model.verifyCodeHint}
          loading={loading}
          onSubmit={onSubmitVerifyCode}
        />
      ) : null}
      <pre className="max-h-[280px] min-h-[132px] overflow-auto whitespace-pre-wrap break-words rounded-[10px] bg-[#101418] px-3 py-3 font-mono text-[12px] leading-5 text-[#d7f8de]">{model.output}</pre>
      {model.error ? (
        <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-3 py-2 text-[12px] leading-5 text-(--destructive)">
          {model.error}
        </div>
      ) : null}
    </div>
  );
}

export function ChannelLoginPanel({
  loading,
  loginView,
  onSubmitVerifyCode,
}: {
  loading: boolean;
  loginView: ChannelLoginView | null;
  onSubmitVerifyCode: (value: string) => void;
}) {
  const model = buildChannelLoginPanelModel(loginView);

  return (
    <div className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-3">
      <ChannelLoginHeader />
      {model.kind === "session" ? (
        <ChannelLoginSession
          loading={loading}
          model={model}
          onSubmitVerifyCode={onSubmitVerifyCode}
        />
      ) : null}
    </div>
  );
}
