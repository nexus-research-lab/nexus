// INPUT: RichMail 本机配对会话、轮询状态与取消/完成回调。
// OUTPUT: 只呈现客户端审批下一步、固定端点和当前等待状态的 plain 弹窗。
// POS: RichMail 配对的人机边界；不显示、复制或接收 Bearer Token。
"use client";

import { Check, Loader2 } from "lucide-react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import type { ConnectorLocalPairingStart } from "@/types/capability/connector";

import type { ConnectorDeviceAuthFailureKind } from "../device-flow/connector-device-auth-poller";
import { useRichMailPairing } from "./use-richmail-pairing";

interface RichMailPairingDialogProps {
  session: ConnectorLocalPairingStart | null;
  onCancel: () => void;
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (message: string, kind: ConnectorDeviceAuthFailureKind) => void;
}

const PAIRING_STEPS = [
  "Nexus 已向本机 RichMail 发起无 Token 配对请求",
  "请在 RichMail 客户端弹窗中批准本次连接",
  "批准后 Nexus 会自动保存 Token 并读取 MCP 工具",
];

export function RichMailPairingDialog({
  onCancel,
  onClose,
  onConnected,
  onError,
  session,
}: RichMailPairingDialogProps) {
  const [message, setMessage] = useResettableState(
    "等待在 RichMail 中批准连接",
    session?.attempt_token ?? null,
  );
  useRichMailPairing({
    onClose,
    onConnected,
    onError,
    onMessage: setMessage,
    session,
  });

  if (!session || typeof document === "undefined") return null;
  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="z-[9999]" onClose={onCancel}>
        <UiDialogShell size="sm">
          <UiDialogHeader
            appearance="plain"
            onClose={onCancel}
            title="连接 RichMail"
          />
          <UiDialogBody className="space-y-4 px-5">
            <div className="flex items-center gap-2 text-xs font-medium text-(--text-muted)">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              <span aria-live="polite">{message}</span>
            </div>
            <ol className="space-y-3 border-y border-(--divider-subtle-color) py-4">
              {PAIRING_STEPS.map((step, index) => (
                <li className="flex items-start gap-3 text-sm leading-5" key={step}>
                  <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-(--surface-interactive-hover-background) text-2xs font-semibold text-(--text-muted)">
                    {index === 0 ? <Check className="h-3 w-3" /> : index + 1}
                  </span>
                  <span className="text-(--text-default)">{step}</span>
                </li>
              ))}
            </ol>
            <div className="text-xs text-(--text-soft)">
              服务地址
              <code className="ml-2 select-all break-all text-(--text-muted)">
                {session.endpoint}
              </code>
            </div>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton onClick={onCancel} size="sm" type="button">
              取消
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
