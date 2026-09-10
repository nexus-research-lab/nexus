// INPUT: RichMail 本机配对会话、轮询状态与取消/完成回调。
// OUTPUT: 本地化的客户端审批下一步、固定端点和当前等待状态，不展示服务端自由文本。
// POS: RichMail 配对的人机边界；不显示、复制或接收 Bearer Token。
"use client";

import { useId } from "react";
import { Check, Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
  "capability.richmail_pairing_requested",
  "capability.richmail_pairing_approve",
  "capability.richmail_pairing_finish",
] satisfies TranslationKey[];

export function RichMailPairingDialog({
  onCancel,
  onClose,
  onConnected,
  onError,
  session,
}: RichMailPairingDialogProps) {
  const { t } = useI18n();
  const dialogId = useId();
  const [status, setStatus] = useResettableState<"pending" | "connected">(
    "pending",
    session?.attempt_token ?? null,
  );
  useRichMailPairing({
    onClose,
    onConnected,
    onError,
    onMessage: setStatus,
    session,
  });

  if (!session || typeof document === "undefined") return null;
  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={`${dialogId}-title`} layer="dialog" onClose={onCancel}>
        <UiDialogShell size="sm" viewport="compactMax">
          <UiDialogHeader
            appearance="plain"
            titleId={`${dialogId}-title`}
            onClose={onCancel}
            title={t("capability.richmail_pairing_title")}
          />
          <UiDialogBody className="space-y-4 px-5" scrollable>
            <div className={cn(
              "flex items-center gap-2",
              getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }),
            )}>
              <Loader2 className={getUiSpinnerClassName({ size: "sm", tone: "muted" })} />
              <span aria-live="polite" className="min-w-0 [overflow-wrap:anywhere]">{t(status === "pending" ? "capability.richmail_pairing_pending" : "capability.richmail_pairing_connected")}</span>
            </div>
            <ol className="space-y-3 border-y border-(--divider-subtle-color) py-4">
              {PAIRING_STEPS.map((step, index) => (
                <li className={cn(
                  "flex items-start gap-3",
                  getUiTypographyClassName({ role: "supporting", tone: "default" }),
                )} key={step}>
                  <UiBadge className="h-5 w-5 px-0" shape="pill" size="xs" tone="default">
                    {index === 0 ? <Check className="h-3 w-3" /> : index + 1}
                  </UiBadge>
                  <span>{t(step)}</span>
                </li>
              ))}
            </ol>
            <div className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
              {t("capability.richmail_pairing_address")}
              <code className={cn(
                "ml-2 select-all break-all",
                getUiTypographyClassName({ role: "code", tone: "muted" }),
              )}>
                {session.endpoint}
              </code>
            </div>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton onClick={onCancel} size="sm" type="button">
              {t("common.cancel")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
