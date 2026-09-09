// INPUT: 当前 RichMail opaque 配对会话与本机服务轮询 API。
// OUTPUT: pending/connected/denied/expired 的单会话轮询生命周期。
// POS: RichMail 配对弹窗的副作用边界；卸载只停止轮询，不重放 start 或删除既有连接。
"use client";

import { useEffect, useRef } from "react";

import { pollConnectorLocalPairingApi } from "@/lib/api/capability/connector-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ConnectorLocalPairingStart } from "@/types/capability/connector";

import type { ConnectorDeviceAuthFailureKind } from "../device-flow/connector-device-auth-poller";

interface RichMailPairingCallbacks {
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (message: string, kind: ConnectorDeviceAuthFailureKind) => void;
  onMessage: (status: "pending" | "connected") => void;
}

interface UseRichMailPairingOptions extends RichMailPairingCallbacks {
  session: ConnectorLocalPairingStart | null;
}

export function useRichMailPairing({
  onClose,
  onConnected,
  onError,
  onMessage,
  session,
}: UseRichMailPairingOptions): void {
  const { t } = useI18n();
  const translateRef = useRef(t);
  translateRef.current = t;
  const callbacksRef = useRef<RichMailPairingCallbacks>({
    onClose,
    onConnected,
    onError,
    onMessage,
  });
  callbacksRef.current = { onClose, onConnected, onError, onMessage };

  useEffect(() => {
    if (!session) return undefined;
    let stopped = false;
    let timeoutId: ReturnType<typeof setTimeout> | null = null;
    const delayMs = Math.max(session.interval || 2, 1) * 1_000;

    const close = () => {
      if (stopped) return;
      stopped = true;
      if (timeoutId !== null) clearTimeout(timeoutId);
      callbacksRef.current.onClose();
    };
    const schedule = () => {
      if (stopped) return;
      timeoutId = setTimeout(() => {
        timeoutId = null;
        void poll();
      }, delayMs);
    };
    const poll = async () => {
      try {
        const result = await pollConnectorLocalPairingApi(
          session.connector_id,
          session.attempt_token,
        );
        if (stopped) return;
        if (result.status === "pending") {
          callbacksRef.current.onMessage("pending");
          schedule();
          return;
        }
        if (result.status === "connected") {
          callbacksRef.current.onMessage("connected");
          close();
          await callbacksRef.current.onConnected(session.connector_id);
          return;
        }
        const fallback = result.status === "expired"
          ? "capability.richmail_pairing_expired"
          : "capability.richmail_pairing_denied";
        callbacksRef.current.onError(
          translateRef.current(fallback),
          "not_connected",
        );
        close();
      } catch {
        if (stopped) return;
        callbacksRef.current.onError(
          translateRef.current("capability.richmail_pairing_unknown"),
          "outcome_unknown",
        );
        close();
      }
    };

    schedule();
    return () => {
      stopped = true;
      if (timeoutId !== null) clearTimeout(timeoutId);
    };
  }, [session]);
}
