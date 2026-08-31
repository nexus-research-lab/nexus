// INPUT: 当前 Device Flow 会话及轮询终态回调。
// OUTPUT: 会话级轮询器生命周期；卸载只停止轮询，不删除 Connector 配置。
// POS: React 与 ConnectorDeviceAuthPoller 之间的窄适配层。
import { useEffect, useRef } from "react";

import { pollConnectorDeviceAuthApi } from "@/lib/api/capability/connector-api";
import type { ConnectorDeviceAuthStart } from "@/types/capability/connector";

import {
  ConnectorDeviceAuthPoller,
  type ConnectorDeviceAuthPollerCallbacks,
} from "./connector-device-auth-poller";

interface UseConnectorDeviceAuthOptions
  extends ConnectorDeviceAuthPollerCallbacks {
  session: ConnectorDeviceAuthStart | null;
}

export function useConnectorDeviceAuth({
  onClose,
  onConnected,
  onError,
  onMessage,
  onNext,
  session,
}: UseConnectorDeviceAuthOptions): void {
  const callbacksRef = useRef<ConnectorDeviceAuthPollerCallbacks>({
    onClose,
    onConnected,
    onError,
    onMessage,
    onNext,
  });
  callbacksRef.current = {
    onClose,
    onConnected,
    onError,
    onMessage,
    onNext,
  };

  useEffect(() => {
    if (!session) {
      return;
    }
    const poller = new ConnectorDeviceAuthPoller(
      session,
      {
        onClose: () => callbacksRef.current.onClose(),
        onConnected: (connectorId) => (
          callbacksRef.current.onConnected(connectorId)
        ),
        onError: (message, kind) => callbacksRef.current.onError(message, kind),
        onMessage: (message) => callbacksRef.current.onMessage(message),
        onNext: (nextSession) => callbacksRef.current.onNext(nextSession),
      },
      pollConnectorDeviceAuthApi,
    );
    poller.start();
    return () => poller.stop();
  }, [session]);
}
