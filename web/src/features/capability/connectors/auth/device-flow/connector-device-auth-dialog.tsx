// INPUT: GitHub 或飞书的 Device Flow 会话、轮询状态与打开/取消动作。
// OUTPUT: 授权码、二维码或跳转动作组成的单任务 plain 弹窗。
// POS: Connector Device Flow 的可见授权面，只显示用户下一步和当前状态。
"use client";

import { Check, Copy, Loader2 } from "lucide-react";
import { useCallback, useEffect, useRef, useId } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { useCopyToClipboard } from "@/shared/lib/react/use-copy-to-clipboard";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import {
  isDesktopBridgeAvailable,
  openDesktopExternalURL,
} from "@/lib/desktop-bridge";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { ConnectorDeviceAuthStart } from "@/types/capability/connector";

import {
  getFeishuDeviceAuthPresentation,
  shouldAutoOpenFeishuUserAuthorization,
} from "../feishu/feishu-app-connection-model";
import type { ConnectorDeviceAuthFailureKind } from "./connector-device-auth-poller";
import { useConnectorDeviceAuth } from "./use-connector-device-auth";

interface ConnectorDeviceAuthDialogProps {
  session: ConnectorDeviceAuthStart | null;
  onCancel: () => void;
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (
    message: string,
    kind?: ConnectorDeviceAuthFailureKind,
  ) => void;
  onNext: (session: ConnectorDeviceAuthStart) => void;
  onOpenWebAuthUrl: (url: string) => boolean;
}

/** GitHub 授权码与飞书分阶段应用扫码/用户链接授权弹窗。 */
export function ConnectorDeviceAuthDialog({
  session,
  onCancel,
  onClose,
  onConnected,
  onError,
  onNext,
  onOpenWebAuthUrl,
}: ConnectorDeviceAuthDialogProps) {
  const { t } = useI18n();
  const titleId = useId();
  const isFeishu = session?.connector_id === "feishu-docx";
  const feishuPresentation = getFeishuDeviceAuthPresentation(session?.stage);
  const autoOpenedDeviceCodeRef = useRef<string | null>(null);
  const activeDeviceCodeRef = useRef<string | null>(null);
  useEffect(() => {
    activeDeviceCodeRef.current = session?.device_code ?? null;
    return () => { activeDeviceCodeRef.current = null; };
  }, [session?.device_code]);
  const [pollingMessage, setPollingMessage] = useResettableState<TranslationKey>(
    isFeishu
      ? feishuPresentation.initialMessage
      : "capability.connector_flow_waiting",
    session?.device_code ?? null,
  );
  useConnectorDeviceAuth({
    onClose,
    onConnected,
    onError,
    onMessage: setPollingMessage,
    onNext,
    session,
  });
  const authUrl = session?.verification_uri_complete
    || session?.verification_uri
    || "";

  useEffect(() => {
    if (
      !authUrl
      || !shouldAutoOpenFeishuUserAuthorization(session)
      || autoOpenedDeviceCodeRef.current === session?.device_code
    ) {
      return;
    }
    autoOpenedDeviceCodeRef.current = session?.device_code ?? null;
    if (!isDesktopBridgeAvailable()) {
      if (onOpenWebAuthUrl(authUrl)) {
        setPollingMessage("capability.connector_flow_feishu_opened");
      } else {
        setPollingMessage("capability.connector_flow_open_manual");
      }
      return;
    }
    const deviceCode = session?.device_code;
    void openDesktopExternalURL(authUrl)
      .then(() => {
        if (activeDeviceCodeRef.current !== deviceCode) return;
        setPollingMessage("capability.connector_flow_feishu_opened");
      })
      .catch(() => {
        if (activeDeviceCodeRef.current !== deviceCode) return;
        setPollingMessage("capability.connector_flow_open_manual");
      });
  }, [authUrl, onOpenWebAuthUrl, session, setPollingMessage]);

  const handleOpenAuthUrl = useCallback(async () => {
    if (!authUrl) {
      onError(t("capability.connector_flow_link_empty"));
      return;
    }
    if (!isDesktopBridgeAvailable()) {
      if (!onOpenWebAuthUrl(authUrl)) {
        onError(t("capability.connector_flow_auth_popup_blocked"));
      }
      return;
    }
    try {
      await openDesktopExternalURL(authUrl);
    } catch {
      onError(t("capability.connector_flow_open_failed"));
    }
  }, [authUrl, onError, onOpenWebAuthUrl, t]);

  if (!session || typeof document === "undefined") {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={titleId} layer="dialog" onClose={onCancel}>
        <UiDialogShell size="sm" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            onClose={onCancel}
            titleId={titleId}
            title={t(isFeishu ? feishuPresentation.title : "capability.connector_flow_github_title")}
          />

          <UiDialogBody className="space-y-4 px-5" scrollable>
            <div className={cn(
              "flex items-center gap-2",
              getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }),
            )}>
              <Loader2 className={getUiSpinnerClassName({ size: "sm", tone: "muted" })} />
              <span aria-live="polite" className="min-w-0 [overflow-wrap:anywhere]">{t(pollingMessage)}</span>
            </div>

            {isFeishu && feishuPresentation.showQRCode ? (
              <UiQRCode
                alt={t(feishuPresentation.qrAlt ?? "capability.connector_flow_qr_alt")}
                payload={authUrl}
              />
            ) : isFeishu ? (
              <p className={cn(
                "py-2",
                getUiTypographyClassName({ role: "supporting", tone: "muted" }),
              )}>
                {t("capability.connector_flow_manual_hint")}
              </p>
            ) : (
              <DeviceAuthorizationCode
                key={session.device_code}
                code={session.user_code}
                onError={onError}
              />
            )}
          </UiDialogBody>

          <UiDialogFooter appearance="plain">
            <UiButton onClick={onCancel} type="button">
              {t("common.cancel")}
            </UiButton>
            <UiButton
              onClick={() => void handleOpenAuthUrl()}
              tone="primary"
              type="button"
              variant="solid"
            >
              {t(isFeishu
                ? feishuPresentation.actionLabel
                : "capability.connector_flow_github_open")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}


function DeviceAuthorizationCode({ code, onError }: {
  code: string;
  onError: ConnectorDeviceAuthDialogProps["onError"];
}) {
  const { t } = useI18n();
  const { copied, copy } = useCopyToClipboard({ feedback_timeout_ms: 1400 });
  const activeRef = useRef(false);
  useEffect(() => {
    activeRef.current = true;
    return () => { activeRef.current = false; };
  }, []);
  const handleCopy = async () => {
    const succeeded = await copy(code);
    if (!succeeded && activeRef.current) onError(t("capability.connector_flow_copy_failed"));
  };
  return (
    <UiPanel padding="md" radius="md" variant="card">
      <div className={getUiTypographyClassName({
        role: "caption",
        tone: "soft",
        weight: "medium",
      })}>{t("capability.connector_flow_code")}</div>
      <div className="mt-2 flex items-center gap-3">
        <code className={cn(
          "min-w-0 flex-1 select-all break-all px-3 py-2.5 text-center",
          getUiTypographyClassName({ role: "objectTitle", tone: "strong" }),
        )}>
          {code}
        </code>
        <UiIconButton
          aria-label={t(copied ? "capability.connector_flow_copied" : "capability.connector_flow_copy")}
          onClick={() => void handleCopy()}
          type="button"
        >
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        </UiIconButton>
      </div>
    </UiPanel>
  );
}
