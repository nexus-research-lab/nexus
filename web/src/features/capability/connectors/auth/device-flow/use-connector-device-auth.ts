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
        onError: (message) => callbacksRef.current.onError(message),
        onMessage: (message) => callbacksRef.current.onMessage(message),
        onNext: (nextSession) => callbacksRef.current.onNext(nextSession),
      },
      pollConnectorDeviceAuthApi,
    );
    poller.start();
    return () => poller.stop();
  }, [session]);
}
