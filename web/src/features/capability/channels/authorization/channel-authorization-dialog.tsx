"use client";

import {
  type FormEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  KeyRound,
  QrCode,
  ShieldCheck,
  TimerReset,
  TriangleAlert,
} from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { UiInput } from "@/shared/ui/form/form-control";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ChannelAuthorizationData } from "@/types/generated/protocol";

interface ChannelAuthorizationDialogProps {
  busy: boolean;
  error: string;
  onCancelAuthorization: () => void;
  onClose: () => void;
  onSubmitCode: (code: string) => void;
  presentation: ChannelAuthorizationData | null;
}

export function ChannelAuthorizationDialog({
  busy,
  error,
  onCancelAuthorization,
  onClose,
  onSubmitCode,
  presentation,
}: ChannelAuthorizationDialogProps) {
  if (!presentation) {
    return null;
  }
  return presentation.kind === "verification_code" ? (
    <ChannelAuthorizationCodeDialog
      busy={busy}
      error={error}
      onCancelAuthorization={onCancelAuthorization}
      onClose={onClose}
      onSubmitCode={onSubmitCode}
      presentation={presentation}
    />
  ) : (
    <ChannelAuthorizationQRCodeDialog
      busy={busy}
      error={error}
      onCancelAuthorization={onCancelAuthorization}
      onClose={onClose}
      presentation={presentation}
    />
  );
}

function ChannelAuthorizationQRCodeDialog({
  busy,
  error,
  onCancelAuthorization,
  onClose,
  presentation,
}: {
  busy: boolean;
  error: string;
  onCancelAuthorization: () => void;
  onClose: () => void;
  presentation: ChannelAuthorizationData;
}) {
  const expiry = useAuthorizationExpiry(presentation.expires_at);
  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[10020]"
        closeOnBackdrop={false}
        describedBy="channel-authorization-description"
        labelledBy="channel-authorization-title"
        onClose={onClose}
      >
        <UiDialogShell className="max-h-[92vh]" size="sm">
          <UiDialogHeader
            icon={<ShieldCheck className="h-6 w-6" />}
            iconClassName="border-[color:color-mix(in_srgb,var(--success)_35%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)"
            onClose={onClose}
            subtitle="仅当前登录会话可见，不会进入智能体消息或工具记录"
            title="安全连接 Channel"
            titleId="channel-authorization-title"
          />
          <UiDialogBody className="space-y-4" scrollable>
            <AuthorizationIdentityStrip
              channelType={presentation.channel_type}
              expiry={expiry}
            />
            <p
              className="text-sm leading-6 text-(--text-default)"
              id="channel-authorization-description"
            >
              {presentation.prompt}
            </p>
            <div className="relative mx-auto w-fit">
              <div className="pointer-events-none absolute -inset-3 rounded-[18px] bg-[radial-gradient(circle_at_center,color-mix(in_srgb,var(--primary)_10%,transparent),transparent_68%)]" />
              <div className="relative">
                <UiQRCode
                  alt={`${presentation.channel_type} 安全授权二维码`}
                  payload={presentation.qr_payload ?? ""}
                  showPayload={false}
                />
              </div>
            </div>
            {error ? <AuthorizationError message={error} /> : null}
            <SecurityBoundaryNote />
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton disabled={busy} onClick={onClose} variant="surface">
              仅关闭弹窗
            </UiButton>
            <UiButton
              disabled={busy}
              onClick={onCancelAuthorization}
              tone="danger"
              variant="solid"
            >
              {busy ? "取消中…" : "取消授权"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function ChannelAuthorizationCodeDialog({
  busy,
  error,
  onCancelAuthorization,
  onClose,
  onSubmitCode,
  presentation,
}: {
  busy: boolean;
  error: string;
  onCancelAuthorization: () => void;
  onClose: () => void;
  onSubmitCode: (code: string) => void;
  presentation: ChannelAuthorizationData;
}) {
  const [code, setCode] = useState("");
  const inputRef = useRef<HTMLInputElement | null>(null);
  const expiry = useAuthorizationExpiry(presentation.expires_at);

  useEffect(() => {
    setCode("");
  }, [presentation.flow_id, presentation.presentation_token]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const value = code.trim();
    if (!value || busy || expiry.expired) {
      return;
    }
    onSubmitCode(value);
    setCode("");
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[10020]"
        closeOnBackdrop={false}
        describedBy="channel-authorization-code-description"
        initialFocusRef={inputRef}
        labelledBy="channel-authorization-code-title"
        onClose={onClose}
      >
        <UiDialogFormShell autoComplete="off" onSubmit={submit} size="sm">
          <UiDialogHeader
            icon={<KeyRound className="h-6 w-6" />}
            iconClassName="border-[color:color-mix(in_srgb,var(--warning)_32%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)"
            onClose={onClose}
            subtitle="验证码直接送入平台授权会话，智能体无法读取"
            title="输入平台验证码"
            titleId="channel-authorization-code-title"
          />
          <UiDialogBody className="space-y-4">
            <AuthorizationIdentityStrip
              channelType={presentation.channel_type}
              expiry={expiry}
            />
            <p
              className="text-sm leading-6 text-(--text-default)"
              id="channel-authorization-code-description"
            >
              {presentation.prompt}
            </p>
            <label className="block space-y-2" htmlFor="channel-authorization-code">
              <span className="text-xs font-semibold uppercase tracking-[0.12em] text-(--text-muted)">
                一次性验证码
              </span>
              <UiInput
                ref={inputRef}
                autoCapitalize="none"
                autoComplete="one-time-code"
                className="h-12 text-center font-mono text-lg tracking-[0.22em]"
                disabled={busy || expiry.expired}
                id="channel-authorization-code"
                inputMode="numeric"
                maxLength={256}
                onChange={(event) => setCode(event.target.value)}
                placeholder="输入验证码"
                spellCheck={false}
                value={code}
                variant="dialog"
              />
            </label>
            {error ? (
              <AuthorizationError message={error} />
            ) : null}
            <SecurityBoundaryNote />
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton disabled={busy} onClick={onClose} variant="surface">
              仅关闭弹窗
            </UiButton>
            <UiButton
              disabled={busy}
              onClick={onCancelAuthorization}
              tone="danger"
              variant="surface"
            >
              取消授权
            </UiButton>
            <UiButton
              disabled={!code.trim() || busy || expiry.expired}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {busy ? "安全提交中…" : "安全提交"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function AuthorizationError({ message }: { message: string }) {
  return (
    <div
      className="flex items-start gap-2 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-3 py-2.5 text-xs leading-5 text-(--destructive)"
      role="alert"
    >
      <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  );
}

function AuthorizationIdentityStrip({
  channelType,
  expiry,
}: {
  channelType: string;
  expiry: AuthorizationExpiry;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--card)_55%,transparent)] px-3 py-2.5">
      <UiBadge icon={<QrCode className="h-3.5 w-3.5" />} size="xs" tone="info">
        {channelType}
      </UiBadge>
      <div
        className={`inline-flex items-center gap-1.5 text-xs font-medium ${
          expiry.expired ? "text-(--destructive)" : "text-(--text-muted)"
        }`}
      >
        <TimerReset className="h-3.5 w-3.5" />
        {expiry.label}
      </div>
    </div>
  );
}

function SecurityBoundaryNote() {
  return (
    <div className="flex items-start gap-2 border-t border-(--divider-subtle-color) pt-3 text-xs leading-5 text-(--text-muted)">
      <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-(--success)" />
      <span>
        Nexus 只把本次材料投递到发起授权的登录用户与业务会话；切换账号、会话失效或服务重启都会拒绝提交。
      </span>
    </div>
  );
}

interface AuthorizationExpiry {
  expired: boolean;
  label: string;
}

function useAuthorizationExpiry(expiresAt: string): AuthorizationExpiry {
  const expiresAtMs = useMemo(() => Date.parse(expiresAt), [expiresAt]);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, []);
  const remainingSeconds = Math.max(0, Math.ceil((expiresAtMs - now) / 1_000));
  if (remainingSeconds <= 0) {
    return { expired: true, label: "授权已过期" };
  }
  const minutes = Math.floor(remainingSeconds / 60);
  const seconds = String(remainingSeconds % 60).padStart(2, "0");
  return {
    expired: false,
    label: `${minutes}:${seconds} 后失效`,
  };
}
