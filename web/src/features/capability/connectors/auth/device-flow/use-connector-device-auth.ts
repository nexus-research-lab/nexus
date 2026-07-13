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
  session,
}: UseConnectorDeviceAuthOptions): void {
  const callbacksRef = useRef<ConnectorDeviceAuthPollerCallbacks>({
    onClose,
    onConnected,
    onError,
    onMessage,
  });
  callbacksRef.current = { onClose, onConnected, onError, onMessage };

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
      },
      pollConnectorDeviceAuthApi,
    );
    poller.start();
    return () => poller.stop();
  }, [session]);
}
