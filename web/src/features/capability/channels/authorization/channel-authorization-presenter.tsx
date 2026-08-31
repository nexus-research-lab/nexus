// INPUT: Channel 授权 WebSocket 展示/结果事件与用户提交动作。
// OUTPUT: 仅内存保存当前展示，向弹窗投影精确提交、取消和受控恢复状态。
// POS: Channel 授权协议与可见弹窗之间的 presenter；不持久化敏感材料、不直显 server message。
"use client";

import { useCallback, useRef, useState } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useWebSocket } from "@/lib/websocket";
import type { ChannelAuthorizationData } from "@/types/generated/protocol";
import { useI18n } from "@/shared/i18n/i18n-context";

import { ChannelAuthorizationDialog } from "./channel-authorization-dialog";
import {
  buildChannelAuthorizationFailure,
  type ChannelAuthorizationAction,
  parseChannelAuthorizationPresentation,
  parseChannelAuthorizationResult,
} from "./channel-authorization-model";

export function ChannelAuthorizationPresenter() {
  const { t } = useI18n();
  const [presentation, setPresentation] = useState<ChannelAuthorizationData | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ReturnType<
    typeof buildChannelAuthorizationFailure
  > | null>(null);
  const pendingActionRef = useRef<ChannelAuthorizationAction | null>(null);

  const handleMessage = useCallback((message: unknown) => {
    const nextPresentation = parseChannelAuthorizationPresentation(message);
    if (nextPresentation) {
      setPresentation(nextPresentation);
      setBusy(false);
      setError(null);
      pendingActionRef.current = null;
      return;
    }
    const result = parseChannelAuthorizationResult(message);
    if (!result) {
      return;
    }
    if (!presentation || presentation.flow_id !== result.flow_id) {
      return;
    }
    if (result.accepted) {
      setPresentation(null);
    } else {
      setError(buildChannelAuthorizationFailure(
        pendingActionRef.current ?? "submit",
        "rejected",
        t,
      ));
    }
    setBusy(false);
    pendingActionRef.current = null;
  }, [presentation, t]);

  const { send } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30_000,
    onMessage: handleMessage,
  });

  const submitCode = useCallback((code: string) => {
    if (
      !presentation
      || presentation.kind !== "verification_code"
      || error?.writeLocked
    ) {
      return;
    }
    setError(null);
    const result = send({
      type: "submit_channel_authorization_code",
      flow_id: presentation.flow_id,
      presentation_token: presentation.presentation_token,
      code,
    });
    if (result.disposition !== "sent") {
      setBusy(false);
      setError(buildChannelAuthorizationFailure(
        "submit",
        "not_sent",
        t,
      ));
      return;
    }
    pendingActionRef.current = "submit";
    setBusy(true);
  }, [error?.writeLocked, presentation, send, t]);

  const cancelAuthorization = useCallback(() => {
    if (!presentation || error?.writeLocked) {
      return;
    }
    setError(null);
    const result = send({
      type: "cancel_channel_authorization",
      flow_id: presentation.flow_id,
      presentation_token: presentation.presentation_token,
    });
    if (result.disposition !== "sent") {
      setBusy(false);
      setError(buildChannelAuthorizationFailure(
        "cancel",
        "not_sent",
        t,
      ));
      return;
    }
    pendingActionRef.current = "cancel";
    setBusy(true);
  }, [error?.writeLocked, presentation, send, t]);

  return (
    <ChannelAuthorizationDialog
      busy={busy}
      error={error}
      onCancelAuthorization={cancelAuthorization}
      onClose={() => {
        setPresentation(null);
        setBusy(false);
        setError(null);
        pendingActionRef.current = null;
      }}
      onSubmitCode={submitCode}
      presentation={presentation}
      writeLocked={error?.writeLocked ?? false}
    />
  );
}
