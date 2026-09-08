// INPUT: GitHub 或飞书的 Device Flow 会话、轮询状态与打开/取消动作。
// OUTPUT: 授权码、二维码或跳转动作组成的单任务 plain 弹窗。
// POS: Connector Device Flow 的可见授权面，只显示用户下一步和当前状态。
"use client";

import { Check, Copy, Loader2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { writeTextToClipboard } from "@/shared/lib/browser/clipboard";
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
  const isFeishu = session?.connector_id === "feishu-docx";
  const feishuPresentation = getFeishuDeviceAuthPresentation(session?.stage);
  const [copied, setCopied] = useState(false);
  const autoOpenedDeviceCodeRef = useRef<string | null>(null);
  const [pollingMessage, setPollingMessage] = useResettableState(
    isFeishu
      ? feishuPresentation.initialMessage
      : "等待授权",
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
        setPollingMessage("已打开飞书授权页，等待确认");
      } else {
        setPollingMessage("授权页未自动打开，请手动继续");
      }
      return;
    }
    void openDesktopExternalURL(authUrl)
      .then(() => {
        setPollingMessage("已打开飞书授权页，等待确认");
      })
      .catch(() => {
        setPollingMessage("授权页未自动打开，请手动继续");
      });
  }, [authUrl, onOpenWebAuthUrl, session, setPollingMessage]);

  const handleCopy = useCallback(async () => {
    if (!session) {
      return;
    }
    if (await writeTextToClipboard(session.user_code)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 1400);
      return;
    }
    onError("复制授权码失败");
  }, [onError, session]);

  const handleOpenAuthUrl = useCallback(async () => {
    if (!authUrl) {
      onError("授权链接为空");
      return;
    }
    if (!isDesktopBridgeAvailable()) {
      if (!onOpenWebAuthUrl(authUrl)) {
        onError("授权窗口被浏览器拦截，请允许弹窗后重试");
      }
      return;
    }
    try {
      await openDesktopExternalURL(authUrl);
    } catch {
      onError("打开授权链接失败");
    }
  }, [authUrl, onError, onOpenWebAuthUrl]);

  if (!session || typeof document === "undefined") {
    return null;
  }

  return (
    <UiDialogPortal>
      <UiDialogBackdrop layer="dialog" onClose={onCancel}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            appearance="plain"
            onClose={onCancel}
            title={isFeishu ? feishuPresentation.title : "连接 GitHub"}
          />

          <UiDialogBody className="space-y-4 px-5">
            <div className={cn(
              "flex items-center gap-2",
              getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }),
            )}>
              <Loader2 className={getUiSpinnerClassName({ size: "sm", tone: "muted" })} />
              <span aria-live="polite">{pollingMessage}</span>
            </div>

            {isFeishu && feishuPresentation.showQRCode ? (
              <UiQRCode
                alt={feishuPresentation.qrAlt ?? "飞书二维码"}
                payload={authUrl}
              />
            ) : isFeishu ? (
              <p className={cn(
                "py-2",
                getUiTypographyClassName({ role: "supporting", tone: "muted" }),
              )}>
                若授权页没有自动打开，请手动继续。
              </p>
            ) : (
              <UiPanel padding="md" radius="md" variant="card">
                <div className={getUiTypographyClassName({
                  role: "caption",
                  tone: "soft",
                  weight: "medium",
                })}>授权码</div>
                <div className="mt-2 flex items-center gap-3">
                  <code className={cn(
                    "min-w-0 flex-1 select-all break-all px-3 py-2.5 text-center",
                    getUiTypographyClassName({ role: "objectTitle", tone: "strong" }),
                  )}>
                    {session.user_code}
                  </code>
                  <UiIconButton
                    aria-label="复制授权码"
                    onClick={() => void handleCopy()}
                    type="button"
                  >
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </UiIconButton>
                </div>
              </UiPanel>
            )}
          </UiDialogBody>

          <UiDialogFooter appearance="plain">
            <UiButton onClick={onCancel} type="button">
              取消
            </UiButton>
            <UiButton
              onClick={() => void handleOpenAuthUrl()}
              tone="primary"
              type="button"
              variant="solid"
            >
              {isFeishu
                ? feishuPresentation.actionLabel
                : "打开 GitHub"}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
