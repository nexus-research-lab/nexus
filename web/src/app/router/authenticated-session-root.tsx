import { Outlet } from "react-router-dom";

import { getDesktopWebsocketProtocols } from "@/config/desktop-runtime";
import { getAgentWsUrl } from "@/config/runtime-endpoints";
import { useWebSocket } from "@/lib/websocket";

export function AuthenticatedAppSessionRoot() {
  const wsUrl = getAgentWsUrl();

  useWebSocket({
    url: wsUrl,
    protocols: getDesktopWebsocketProtocols(),
    autoConnect: true,
    reconnect: true,
    heartbeatInterval: 30000,
  });

  return <Outlet />;
}
