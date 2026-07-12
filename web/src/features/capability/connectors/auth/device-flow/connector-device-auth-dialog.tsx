"use client";

import { Check, Copy, ExternalLink, Github, Loader2 } from "lucide-react";
import { useCallback, useState } from "react";

import { writeTextToClipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiPanel } from "@/shared/ui/panel";
import type { ConnectorDeviceAuthStart } from "@/types/capability/connector";

import { useConnectorDeviceAuth } from "./use-connector-device-auth";

interface ConnectorDeviceAuthDialogProps {
  session: ConnectorDeviceAuthStart | null;
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (message: string) => void;
}

/** 桌面 GitHub Device Flow 授权弹窗。 */
export function ConnectorDeviceAuthDialog({
  session,
  onClose,
  onConnected,
  onError,
}: ConnectorDeviceAuthDialogProps) {
  const [copied, setCopied] = useState(false);
  const [pollingMessage, setPollingMessage] = useResettableState(
    "等待 GitHub 授权确认",
    session?.device_code ?? null,
  );
  useConnectorDeviceAuth({
    onClose,
    onConnected,
    onError,
    onMessage: setPollingMessage,
    session,
  });

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

  if (!session || typeof document === "undefined") {
    return null;
  }

  const authUrl = session.verification_uri_complete || session.verification_uri;
  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onClose}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            icon={<Github className="h-5 w-5" />}
            onClose={onClose}
            subtitle="在 GitHub 输入授权码完成连接。"
            title="连接 GitHub"
          />

          <UiDialogBody className="space-y-4">
            <UiPanel padding="sm" variant="inset">
              <div className="flex items-center gap-2 text-[13px] font-medium text-(--text-default)">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span aria-live="polite">{pollingMessage}</span>
              </div>
            </UiPanel>

            <UiPanel padding="md">
              <div className="text-[11px] font-semibold uppercase text-(--text-soft)">GitHub code</div>
              <div className="mt-2 flex items-center gap-3">
                <code className="min-w-0 flex-1 select-all break-all rounded-[14px] bg-transparent px-3 py-2.5 text-center text-[24px] font-black text-(--text-strong)">
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
          </UiDialogBody>

          <UiDialogFooter>
            <UiButton onClick={onClose} type="button">
              取消
            </UiButton>
            <UiButton
              onClick={() => window.open(authUrl, "_blank", "noopener,noreferrer")}
              tone="primary"
              type="button"
              variant="solid"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              打开 GitHub
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
