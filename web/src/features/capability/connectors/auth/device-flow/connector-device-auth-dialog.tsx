"use client";

import { Check, Copy, ExternalLink, Github, Loader2, QrCode } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
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
import { UiQRCode } from "@/shared/ui/display/qr-code";
import { UiPanel } from "@/shared/ui/panel";
import type { ConnectorDeviceAuthStart } from "@/types/capability/connector";

import {
  getFeishuDeviceAuthPresentation,
  shouldAutoOpenFeishuUserAuthorization,
} from "../feishu/feishu-app-connection-model";
import { useConnectorDeviceAuth } from "./use-connector-device-auth";

interface ConnectorDeviceAuthDialogProps {
  session: ConnectorDeviceAuthStart | null;
  onCancel: () => void;
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (message: string) => void;
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
      : "等待 GitHub 授权确认",
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
        setPollingMessage("已打开飞书授权页面，等待当前用户确认");
      } else {
        setPollingMessage("浏览器阻止了自动弹窗，请点击下方“继续飞书授权”");
      }
      return;
    }
    void openDesktopExternalURL(authUrl)
      .then(() => {
        setPollingMessage("已打开飞书授权页面，等待当前用户确认");
      })
      .catch(() => {
        setPollingMessage("自动打开失败，请点击下方“继续飞书授权”");
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
      <UiDialogBackdrop className="z-[9999]" onClose={onCancel}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            icon={isFeishu
              ? feishuPresentation.showQRCode
                ? <QrCode className="h-5 w-5" />
                : <ExternalLink className="h-5 w-5" />
              : <Github className="h-5 w-5" />}
            onClose={onCancel}
            subtitle={isFeishu
              ? feishuPresentation.subtitle
              : "在 GitHub 输入授权码完成连接。"}
            title={isFeishu ? feishuPresentation.title : "连接 GitHub"}
          />

          <UiDialogBody className="space-y-4">
            <UiPanel padding="sm" variant="inset">
              <div className="flex items-center gap-2 text-sm font-medium text-(--text-default)">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span aria-live="polite">{pollingMessage}</span>
              </div>
            </UiPanel>

            {isFeishu && feishuPresentation.showQRCode ? (
              <UiQRCode
                alt={feishuPresentation.qrAlt ?? "飞书二维码"}
                payload={authUrl}
              />
            ) : isFeishu ? (
              <UiPanel padding="md">
                <div className="flex flex-col items-center gap-3 py-4 text-center">
                  <span className="flex h-11 w-11 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-muted)">
                    <ExternalLink className="h-5 w-5 text-(--icon-default)" />
                  </span>
                  <div>
                    <div className="text-sm font-semibold text-(--text-strong)">
                      在浏览器中继续授权
                    </div>
                    <p className="mt-1 max-w-sm text-sm leading-6 text-(--text-muted)">
                      Nexus 已取得应用信息并会自动尝试打开授权页面；如浏览器没有弹出窗口，请点击下方蓝色按钮继续。
                    </p>
                  </div>
                </div>
              </UiPanel>
            ) : (
              <UiPanel padding="md">
                <div className="text-xs font-semibold uppercase text-(--text-soft)">GitHub code</div>
                <div className="mt-2 flex items-center gap-3">
                  <code className="min-w-0 flex-1 select-all break-all rounded-[10px] bg-transparent px-3 py-2.5 text-center text-lg font-semibold text-(--text-strong)">
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

          <UiDialogFooter>
            <UiButton onClick={onCancel} type="button">
              取消
            </UiButton>
            <UiButton
              onClick={() => void handleOpenAuthUrl()}
              tone="primary"
              type="button"
              variant="solid"
            >
              <ExternalLink className="h-3.5 w-3.5" />
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
