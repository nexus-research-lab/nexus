// INPUT: Requests made by the real App entry in an isolated native QA process.
// OUTPUT: Fixed read-only directory/auth/provider snapshots and an idle local event socket.
// POS: App-shell fixture boundary; no production imports, backend forwarding or mutation support.

import { WebSocketServer } from "ws";

const AGENTS = [
  { id: "qa-main", name: "Nexus", description: "Local UI fixture" },
  { id: "qa-reader", name: "Research", description: "Read-only directory fixture" },
];

const READS = new Map([
  ["/nexus/v1/auth/status", { auth_required: false, authenticated: true,
    password_login_enabled: false, setup_required: false, username: "ui-fixture",
    user_id: "ui-fixture", display_name: "UI Fixture", role: "admin", auth_method: "desktop_local" }],
  ["/nexus/v1/runtime/options", { default_agent_id: "qa-main" }],
  ["/nexus/v1/launcher/bootstrap", { agents: AGENTS, rooms: [], conversations: [] }],
  ["/nexus/v1/settings/providers/options", { default_selection: { provider: "qa", model: "qa-model" },
    items: [{ provider: "qa", name: "Local UI Fixture", models: [{ model_id: "qa-model", name: "Fixture" }] }] }],
]);

export function appShellRead(method, pathname) {
  return method === "GET" && READS.has(pathname) ? { data: READS.get(pathname) } : null;
}

// Runs before the real App bootstrap. This seeds only isolated UI preferences,
// not React state or application modules; events still reach production handlers.
export const APP_SHELL_INIT_SCRIPT = `(() => {
  if (!['http:', 'https:'].includes(location.protocol)) return;
  const route = new URL(new URL(location.href).searchParams.get('desktop_route') || location.pathname + location.search, location.origin);
  const theme = route.searchParams.get('theme') || localStorage.getItem('nexus-theme') || 'light';
  const locale = route.searchParams.get('locale') || localStorage.getItem('nexus-locale') || 'en';
  localStorage.setItem('nexus-theme', theme);
  localStorage.setItem('nexus-locale', locale);
  localStorage.setItem('nexus:onboarding:dismissed-tours', JSON.stringify({'launcher-guide':true,'sidebar-navigation':true}));
  localStorage.setItem('nexus:sidebar-onboarding-dismissed', 'true');
  window.qaEvents = []; window.qaErrors = [];
  for (const type of ['click','keydown','input']) document.addEventListener(type, e => qaEvents.push({type,key:e.key || null,trusted:e.isTrusted}),true);
  window.addEventListener('error',e => qaErrors.push(e.message));
  window.addEventListener('unhandledrejection',e => qaErrors.push(String(e.reason)));
})()`;

export function attachAppShellSocket(httpServer, observations) {
  const sockets = new WebSocketServer({ noServer: true });
  httpServer.on("upgrade", (request, socket, head) => {
    if (new URL(request.url, "http://localhost").pathname !== "/nexus/v1/chat/ws") return;
    sockets.handleUpgrade(request, socket, head, (client) => {
      observations.socketConnections += 1;
      client.on("message", (raw) => {
        let message;
        try { message = JSON.parse(raw.toString()); } catch { message = {}; }
        const type = message?.type;
        if (!["ping", "subscribe_app_events", "unsubscribe_app_events"].includes(type)) {
          observations.rejected.push(`WS ${String(type)}`);
          client.close(1008, "Unsupported fixture command");
          return;
        }
        observations.socketMessages.push(type);
        if (type === "ping") client.send(JSON.stringify({ event_type: "pong" }));
      });
    });
  });
  return () => { for (const client of sockets.clients) client.terminate(); sockets.close(); };
}
