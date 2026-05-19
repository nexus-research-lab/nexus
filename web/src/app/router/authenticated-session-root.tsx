import { Outlet } from "react-router-dom";

import { get_desktop_websocket_protocols } from "@/config/desktop-runtime";
import { get_agent_ws_url } from "@/config/options";
import { useWebSocket } from "@/lib/websocket";

export function AuthenticatedAppSessionRoot() {
  const ws_url = get_agent_ws_url();

  useWebSocket({
    url: ws_url,
    protocols: get_desktop_websocket_protocols(),
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
  });

  return <Outlet />;
}
