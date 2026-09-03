// INPUT: 当前 RichMail opaque 配对会话与本机服务轮询 API。
// OUTPUT: pending/connected/denied/expired 的单会话轮询生命周期。
// POS: RichMail 配对弹窗的副作用边界；卸载只停止轮询，不重放 start 或删除既有连接。
"use client";

import { useEffect, useRef } from "react";

import { pollConnectorLocalPairingApi } from "@/lib/api/capability/connector-api";
import { getErrorMessage } from "@/lib/error-message";
import type { ConnectorLocalPairingStart } from "@/types/capability/connector";

import type { ConnectorDeviceAuthFailureKind } from "../device-flow/connector-device-auth-poller";

interface RichMailPairingCallbacks {
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (message: string, kind: ConnectorDeviceAuthFailureKind) => void;
  onMessage: (message: string) => void;
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
          callbacksRef.current.onMessage(
            result.message || "等待在 RichMail 中批准连接",
          );
          schedule();
          return;
        }
        if (result.status === "connected") {
          callbacksRef.current.onMessage(result.message || "RichMail 已连接");
          close();
          await callbacksRef.current.onConnected(session.connector_id);
          return;
        }
        const fallback = result.status === "expired"
          ? "RichMail 配对请求已过期"
          : "RichMail 未批准本次连接";
        callbacksRef.current.onError(
          result.message || fallback,
          "not_connected",
        );
        close();
      } catch (error) {
        if (stopped) return;
        callbacksRef.current.onError(
          getErrorMessage(error, "RichMail 配对结果暂时无法确认"),
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
