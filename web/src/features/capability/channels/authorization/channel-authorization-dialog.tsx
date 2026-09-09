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
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
import type { ChannelAuthorizationFailure } from "./channel-authorization-model";

interface ChannelAuthorizationDialogProps {
  busy: boolean;
  error: ChannelAuthorizationFailure | null;
  onCancelAuthorization: () => void;
  onClose: () => void;
  onSubmitCode: (code: string) => void;
  presentation: ChannelAuthorizationData | null;
  writeLocked: boolean;
}

export function ChannelAuthorizationDialog({
  busy,
  error,
  onCancelAuthorization,
  onClose,
  onSubmitCode,
  presentation,
  writeLocked,
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
      writeLocked={writeLocked}
    />
  ) : (
    <ChannelAuthorizationQRCodeDialog
      busy={busy}
      error={error}
      onCancelAuthorization={onCancelAuthorization}
      onClose={onClose}
      presentation={presentation}
      writeLocked={writeLocked}
    />
  );
}

function ChannelAuthorizationQRCodeDialog({
  busy,
  error,
  onCancelAuthorization,
  onClose,
  presentation,
  writeLocked,
}: {
  busy: boolean;
  error: ChannelAuthorizationFailure | null;
  onCancelAuthorization: () => void;
  onClose: () => void;
  presentation: ChannelAuthorizationData;
  writeLocked: boolean;
}) {
  const { t } = useI18n();
  const expiry = useAuthorizationExpiry(presentation.expires_at);
  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        closeOnBackdrop={false}
        describedBy="channel-authorization-description"
        labelledBy="channel-authorization-title"
        layer="dialogInteraction"
        onClose={onClose}
      >
        <UiDialogShell size="sm" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("capability.channel_auth_connect")}
            titleId="channel-authorization-title"
          />
          <UiDialogBody className="space-y-4 px-5" scrollable>
            <AuthorizationIdentityStrip
              channelType={presentation.channel_type}
              expiry={expiry}
            />
            <p
              className={getUiTypographyClassName({ role: "body", tone: "default" })}
              id="channel-authorization-description"
            >
              {presentation.prompt}
            </p>
            <div className="mx-auto w-fit">
              <UiQRCode
                alt={t("capability.channel_auth_qr_alt", { channel: presentation.channel_type })}
                payload={presentation.qr_payload ?? ""}
                showPayload={false}
              />
            </div>
            {error ? <AuthorizationError failure={error} /> : null}
            {expiry.expired ? <AuthorizationExpired /> : null}
            <SecurityBoundaryNote>{t("capability.channel_auth_boundary")}</SecurityBoundaryNote>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} variant="surface">
              {t("common.close")}
            </UiButton>
            <UiButton
              disabled={busy || writeLocked}
              onClick={onCancelAuthorization}
              tone="danger"
              variant="solid"
            >
              {t(busy ? "capability.channel_auth_cancelling" : "capability.channel_auth_cancel")}
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
  writeLocked,
}: {
  busy: boolean;
  error: ChannelAuthorizationFailure | null;
  onCancelAuthorization: () => void;
  onClose: () => void;
  onSubmitCode: (code: string) => void;
  presentation: ChannelAuthorizationData;
  writeLocked: boolean;
}) {
  const [code, setCode] = useState("");
  const inputRef = useRef<HTMLInputElement | null>(null);
  const { t } = useI18n();
  const expiry = useAuthorizationExpiry(presentation.expires_at);

  useEffect(() => {
    setCode("");
  }, [presentation.flow_id, presentation.presentation_token]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const value = code.trim();
    if (!value || busy || expiry.expired || writeLocked) {
      return;
    }
    onSubmitCode(value);
    setCode("");
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        closeOnBackdrop={false}
        describedBy="channel-authorization-code-description"
        initialFocusRef={inputRef}
        layer="dialogInteraction"
        labelledBy="channel-authorization-code-title"
        onClose={onClose}
      >
        <UiDialogFormShell autoComplete="off" onSubmit={submit} size="sm" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("capability.channel_auth_code_title")}
            titleId="channel-authorization-code-title"
          />
          <UiDialogBody className="space-y-4 px-5" scrollable>
            <AuthorizationIdentityStrip
              channelType={presentation.channel_type}
              expiry={expiry}
            />
            <p
              className={getUiTypographyClassName({ role: "body", tone: "default" })}
              id="channel-authorization-code-description"
            >
              {presentation.prompt}
            </p>
            <UiField htmlFor="channel-authorization-code" label={t("capability.channel_auth_code")}>
              <UiInput
                ref={inputRef}
                autoCapitalize="none"
                autoComplete="one-time-code"
                disabled={busy || expiry.expired || writeLocked}
                id="channel-authorization-code"
                inputMode="numeric"
                maxLength={256}
                onChange={(event) => setCode(event.target.value)}
                placeholder={t("capability.channel_auth_code_title")}
                spellCheck={false}
                textRole="verification"
                value={code}
                variant="dialog"
              />
            </UiField>
            {error ? (
              <AuthorizationError failure={error} />
            ) : null}
            {expiry.expired ? <AuthorizationExpired /> : null}
            <SecurityBoundaryNote>{t("capability.channel_auth_code_boundary")}</SecurityBoundaryNote>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton disabled={busy} onClick={onClose} variant="surface">
              {t("common.close")}
            </UiButton>
            <UiButton
              disabled={busy || writeLocked}
              onClick={onCancelAuthorization}
              tone="danger"
              variant="surface"
            >
              {t("capability.channel_auth_cancel")}
            </UiButton>
            <UiButton
              disabled={!code.trim() || busy || expiry.expired || writeLocked}
              tone="primary"
              type="submit"
              variant="solid"
            >
              {t(busy ? "capability.channel_auth_submitting" : "capability.channel_auth_submit")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function AuthorizationError({
  failure,
}: {
  failure: ChannelAuthorizationFailure;
}) {
  return (
    <UiInlineNotice
      message={(
        <RecoverySummary impact={failure.impact} nextStep={failure.nextStep} />
      )}
      title={failure.title}
      tone="danger"
    />
  );
}

function AuthorizationExpired() {
  const { t } = useI18n();
  return (
    <UiInlineNotice
      message={(
        <RecoverySummary
          impact={t("capability.channel_authorization_expired_impact")}
          nextStep={t("capability.channel_authorization_expired_next_step")}
        />
      )}
      title={t("capability.channel_authorization_expired_title")}
      tone="warning"
    />
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
        className={cn(
          "inline-flex items-center gap-1.5",
          getUiTypographyClassName({
            role: "caption",
            tone: expiry.expired ? "danger" : "muted",
            weight: "medium",
          }),
        )}
      >
        <TimerReset className="h-3.5 w-3.5" />
        {expiry.label}
      </div>
    </div>
  );
}

function SecurityBoundaryNote({ children }: { children: string }) {
  return (
    <p className={cn(
      "border-t border-(--divider-subtle-color) pt-3",
      getUiTypographyClassName({ role: "caption", tone: "muted" }),
    )}>
      {children}
    </p>
  );
}

interface AuthorizationExpiry {
  expired: boolean;
  label: string;
}

function useAuthorizationExpiry(expiresAt: string): AuthorizationExpiry {
  const { t } = useI18n();
  const expiresAtMs = useMemo(() => Date.parse(expiresAt), [expiresAt]);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, []);
  const remainingSeconds = Math.max(0, Math.ceil((expiresAtMs - now) / 1_000));
  if (remainingSeconds <= 0) {
    return { expired: true, label: t("capability.channel_auth_expired") };
  }
  const minutes = Math.floor(remainingSeconds / 60);
  const seconds = String(remainingSeconds % 60).padStart(2, "0");
  return {
    expired: false,
    label: t("capability.channel_auth_remaining", { time: `${minutes}:${seconds}` }),
  };
}
