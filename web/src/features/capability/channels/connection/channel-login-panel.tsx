"use client";

import { useEffect, useState } from "react";
import {
  CircleCheck,
  QrCode,
  Terminal,
  TriangleAlert,
} from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { ChannelLoginView } from "@/lib/api/channel-api";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { UiInput } from "@/shared/ui/form-control";
import {
  channelLoginStatusLabel,
  channelLoginStatusTone,
  isChannelLoginRunning,
} from "./channel-connection-model";

const LOGIN_STATUS_ICONS: Record<string, typeof Terminal> = {
  error: TriangleAlert,
  expired: TriangleAlert,
  succeeded: CircleCheck,
};

function LoginQRCode({ payload }: { payload?: string }) {
  const value = payload?.trim() || "";
  const [generatedImageUrl, setGeneratedImageUrl] = useResettableState("", value);
  const imageUrl = value.startsWith("data:image/") ? value : generatedImageUrl;

  useEffect(() => {
    if (!value || value.startsWith("data:image/")) {
      return;
    }
    let cancelled = false;
    void import("qrcode")
      .then((module) => module.toDataURL(value, {
        errorCorrectionLevel: "M",
        margin: 1,
        scale: 7,
        width: 220,
      }))
      .then((url) => {
        if (!cancelled) {
          setGeneratedImageUrl(url);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setGeneratedImageUrl("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [setGeneratedImageUrl, value]);

  if (!payload) {
    return null;
  }

  return (
    <div className="flex flex-col items-center gap-2 rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
      {imageUrl ? (
        <img
          alt="微信扫码登录二维码"
          className="h-[220px] w-[220px] rounded-[8px] bg-(--surface-paper-background) p-2"
          src={imageUrl}
        />
      ) : (
        <div className="flex h-[220px] w-[220px] items-center justify-center rounded-[8px] bg-(--surface-paper-background) p-4 text-center text-[12px] leading-5 text-(--surface-paper-muted)">
          二维码生成失败，请使用下方链接
        </div>
      )}
      <code className="max-w-full truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
        {payload}
      </code>
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
  const [verifyCode, setVerifyCode] = useState("");
  const running = isChannelLoginRunning(loginView);
  const output = loginView?.output?.trimEnd() || (running ? "等待 iLink 扫码状态..." : "");
  const status = loginView?.status || "";
  const statusTone = channelLoginStatusTone(status);
  const StatusIcon = LOGIN_STATUS_ICONS[status] ?? Terminal;
  const verifyRequired = status === "verify_code_required";

  return (
    <div className="rounded-[14px] border border-(--divider-subtle-color) bg-transparent px-4 py-3">
      <div className="min-w-0">
        <div className="flex items-center gap-2 text-[13px] font-semibold text-(--text-strong)">
          <QrCode className="h-4 w-4 text-(--primary)" />
          扫码登录
        </div>
        <p className="mt-1 text-[12px] leading-5 text-(--text-muted)">
          Nexus 会先保存当前配置，再请求腾讯 iLink Bot API 生成二维码。
        </p>
      </div>

      {loginView ? (
        <div className="mt-3 space-y-3">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <UiBadge size="xs" tone={statusTone}>
              <StatusIcon className="mr-1 h-3 w-3" />
              {channelLoginStatusLabel(status)}
            </UiBadge>
            <code className="min-w-0 truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
              {loginView.account_id || loginView.command || "Nexus iLink QR login"}
            </code>
          </div>
          <LoginQRCode payload={loginView.qr_payload} />
          {verifyRequired ? (
            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-3 py-3">
              <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">
                {loginView.verify_code_hint || "输入手机微信显示的数字"}
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
                  onClick={() => {
                    onSubmitVerifyCode(verifyCode);
                    setVerifyCode("");
                  }}
                  size="sm"
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  提交
                </UiButton>
              </div>
            </div>
          ) : null}
          <pre className="max-h-[280px] min-h-[132px] overflow-auto whitespace-pre-wrap break-words rounded-[10px] bg-[#101418] px-3 py-3 font-mono text-[12px] leading-5 text-[#d7f8de]">{output}</pre>
          {loginView.error ? (
            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-3 py-2 text-[12px] leading-5 text-(--destructive)">
              {loginView.error}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
