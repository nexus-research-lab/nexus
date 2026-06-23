"use client";

import { useEffect, useState } from "react";
import {
  CircleCheck,
  QrCode,
  Terminal,
  TriangleAlert,
} from "lucide-react";

import { ChannelLoginView } from "@/lib/api/channel-api";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { UiInput } from "@/shared/ui/form-control";
import { is_channel_login_running } from "./channel-model";

function channel_login_status_label(status: string) {
  switch (status) {
  case "running":
    return "等待扫码";
  case "verify_code_required":
    return "需要验证码";
  case "succeeded":
    return "登录完成";
  case "expired":
    return "已超时";
  case "cancelled":
    return "已取消";
  case "error":
    return "登录失败";
  default:
    return status || "未启动";
  }
}

function channel_login_status_tone(status: string): "default" | "success" | "warning" | "danger" | "info" {
  switch (status) {
  case "succeeded":
    return "success";
  case "running":
    return "info";
  case "verify_code_required":
    return "warning";
  case "expired":
  case "cancelled":
    return "warning";
  case "error":
    return "danger";
  default:
    return "default";
  }
}

function LoginQRCode({ payload }: { payload?: string }) {
  const [image_url, set_image_url] = useState("");

  useEffect(() => {
    const value = payload?.trim() || "";
    if (!value) {
      set_image_url("");
      return;
    }
    if (value.startsWith("data:image/")) {
      set_image_url(value);
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
          set_image_url(url);
        }
      })
      .catch(() => {
        if (!cancelled) {
          set_image_url("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [payload]);

  if (!payload) {
    return null;
  }

  return (
    <div className="flex flex-col items-center gap-2 rounded-[12px] border border-(--divider-subtle-color) px-4 py-4">
      {image_url ? (
        <img
          alt="微信扫码登录二维码"
          className="h-[220px] w-[220px] rounded-[8px] bg-(--surface-paper-background) p-2"
          src={image_url}
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
  login_view,
  on_submit_verify_code,
}: {
  loading: boolean;
  login_view: ChannelLoginView | null;
  on_submit_verify_code: (value: string) => void;
}) {
  const [verify_code, set_verify_code] = useState("");
  const running = is_channel_login_running(login_view);
  const output = login_view?.output?.trimEnd() || (running ? "等待 iLink 扫码状态..." : "");
  const status = login_view?.status || "";
  const status_tone = channel_login_status_tone(status);
  const StatusIcon = status === "succeeded" ? CircleCheck : status === "error" || status === "expired" ? TriangleAlert : Terminal;
  const verify_required = status === "verify_code_required";

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

      {login_view ? (
        <div className="mt-3 space-y-3">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <UiBadge size="xs" tone={status_tone}>
              <StatusIcon className="mr-1 h-3 w-3" />
              {channel_login_status_label(status)}
            </UiBadge>
            <code className="min-w-0 truncate rounded-[8px] border border-(--divider-subtle-color) px-2 py-1 text-[11px] text-(--text-muted)">
              {login_view.account_id || login_view.command || "Nexus iLink QR login"}
            </code>
          </div>
          <LoginQRCode payload={login_view.qr_payload} />
          {verify_required ? (
            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_8%,transparent)] px-3 py-3">
              <div className="mb-2 text-[12px] font-semibold text-(--text-strong)">
                {login_view.verify_code_hint || "输入手机微信显示的数字"}
              </div>
              <div className="flex gap-2">
                <UiInput
                  onChange={(event) => set_verify_code(event.target.value)}
                  placeholder="验证码"
                  value={verify_code}
                  variant="dialog"
                />
                <UiButton
                  disabled={!verify_code.trim() || loading}
                  onClick={() => {
                    on_submit_verify_code(verify_code);
                    set_verify_code("");
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
          {login_view.error ? (
            <div className="rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-3 py-2 text-[12px] leading-5 text-(--destructive)">
              {login_view.error}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
