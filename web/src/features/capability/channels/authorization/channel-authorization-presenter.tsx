"use client";

import { useCallback, useState } from "react";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useWebSocket } from "@/lib/websocket";
import type { ChannelAuthorizationData } from "@/types/generated/protocol";

import { ChannelAuthorizationDialog } from "./channel-authorization-dialog";
import {
  parseChannelAuthorizationPresentation,
  parseChannelAuthorizationResult,
} from "./channel-authorization-model";

export function ChannelAuthorizationPresenter() {
  const [presentation, setPresentation] = useState<ChannelAuthorizationData | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const handleMessage = useCallback((message: unknown) => {
    const nextPresentation = parseChannelAuthorizationPresentation(message);
    if (nextPresentation) {
      setPresentation(nextPresentation);
      setBusy(false);
      setError("");
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
      setError(result.message);
    }
    setBusy(false);
  }, [presentation]);

  const { send } = useWebSocket({
    url: getAgentWsUrl(),
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30_000,
    onMessage: handleMessage,
  });

  const submitCode = useCallback((code: string) => {
    if (!presentation || presentation.kind !== "verification_code") {
      return;
    }
    setError("");
    const result = send({
      type: "submit_channel_authorization_code",
      flow_id: presentation.flow_id,
      presentation_token: presentation.presentation_token,
      code,
    });
    if (result.disposition !== "sent") {
      setBusy(false);
      setError("安全连接当前不可用，验证码未发送；请恢复连接后重试。");
      return;
    }
    setBusy(true);
  }, [presentation, send]);

  const cancelAuthorization = useCallback(() => {
    if (!presentation) {
      return;
    }
    setError("");
    const result = send({
      type: "cancel_channel_authorization",
      flow_id: presentation.flow_id,
      presentation_token: presentation.presentation_token,
    });
    if (result.disposition !== "sent") {
      setBusy(false);
      setError("安全连接当前不可用，授权尚未取消；请恢复连接后重试。");
      return;
    }
    setBusy(true);
  }, [presentation, send]);

  return (
    <ChannelAuthorizationDialog
      busy={busy}
      error={error}
      onCancelAuthorization={cancelAuthorization}
      onClose={() => {
        setPresentation(null);
        setBusy(false);
        setError("");
      }}
      onSubmitCode={submitCode}
      presentation={presentation}
    />
  );
}
