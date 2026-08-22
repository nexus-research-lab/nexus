// INPUT: 当前 Channel 授权展示、过期时间、提交/取消与本地关闭动作。
// OUTPUT: 二维码或验证码的 plain 授权弹窗，保留必要的会话边界与失效提示。
// POS: Channel 敏感授权材料的唯一可见面，不把安全实现细节扩写成教程或状态卡。
"use client";

import {
  type FormEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  TimerReset,
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
            appearance="plain"
            onClose={onClose}
            title="连接频道"
            titleId="channel-authorization-title"
          />
          <UiDialogBody className="space-y-4 px-5" scrollable>
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
            <div className="mx-auto w-fit">
              <UiQRCode
                alt={`${presentation.channel_type} 授权二维码`}
                payload={presentation.qr_payload ?? ""}
                showPayload={false}
              />
            </div>
            {error ? <AuthorizationError message={error} /> : null}
            <SecurityBoundaryNote>授权信息只用于本次连接。</SecurityBoundaryNote>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} variant="surface">
              关闭
            </UiButton>
            <UiButton
              disabled={busy}
              onClick={onCancelAuthorization}
              tone="danger"
              variant="solid"
            >
              {busy ? "取消中…" : "取消连接"}
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
            appearance="plain"
            onClose={onClose}
            title="输入验证码"
            titleId="channel-authorization-code-title"
          />
          <UiDialogBody className="space-y-4 px-5">
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
              <span className="text-xs font-medium text-(--text-muted)">
                验证码
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
            <SecurityBoundaryNote>验证码只用于本次连接。</SecurityBoundaryNote>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} variant="surface">
              关闭
            </UiButton>
            <UiButton
              disabled={busy}
              onClick={onCancelAuthorization}
              tone="danger"
              variant="surface"
            >
              取消连接
            </UiButton>
            <UiButton
              disabled={!code.trim() || busy || expiry.expired}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {busy ? "提交中…" : "提交"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function AuthorizationError({ message }: { message: string }) {
  return (
    <p
      className="text-xs leading-5 text-(--destructive)"
      role="alert"
    >
      {message}
    </p>
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
    <div className="flex flex-wrap items-center justify-between gap-2">
      <UiBadge size="xs" tone="default">
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

function SecurityBoundaryNote({ children }: { children: string }) {
  return (
    <p className="border-t border-(--divider-subtle-color) pt-3 text-xs leading-5 text-(--text-muted)">
      {children}
    </p>
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
