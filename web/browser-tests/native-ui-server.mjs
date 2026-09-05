// INPUT: A private local port and native launcher routes from the UI harness.
// OUTPUT: Current-source Gallery or real App shell, with isolated read fixtures and rejected mutations.
// POS: Native UI fixture server; never participates in product builds or backend requests.

import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { appShellRead, APP_SHELL_INIT_SCRIPT, attachAppShellSocket } from "./native-ui-app-fixtures.mjs";

const root = fileURLToPath(new URL("..", import.meta.url));
const port = Number(process.env.NEXUS_UI_TEST_PORT);
let rejectedBusinessRequests = 0;
const appShell = process.env.NEXUS_UI_TEST_SURFACE === "app-shell";
const observations = { reads: [], rejected: [], socketConnections: 0, socketMessages: [] };
let closeSocket = () => {};
if (!Number.isInteger(port) || port < 1024 || port > 65535 || port === 34343) throw new Error("An isolated test port is required");
const server = await createServer({
  root, mode: "native-ui-test", cacheDir: path.join(root, "node_modules/.vite-native-ui-test"),
  server: { host: "127.0.0.1", port, strictPort: true },
  plugins: [{
    name: "native-ui-fixtures",
    configResolved(config) {
      // Vite's development proxy must never forward either HTTP or WS to a
      // user's running backend. Both suites own their entire transport boundary.
      config.server.proxy = undefined;
    },
    transformIndexHtml() {
      return appShell ? [{ tag: "script", children: APP_SHELL_INIT_SCRIPT, injectTo: "head-prepend" }] : [];
    },
    configureServer(vite) {
      if (appShell) closeSocket = attachAppShellSocket(vite.httpServer, observations);
      vite.middlewares.use((request, response, next) => {
        const url = new URL(request.url ?? "/", `http://127.0.0.1:${port}`);
        if (url.pathname.startsWith("/nexus/v1/") || url.pathname.startsWith("/auth/")) {
          const fixture = appShell && appShellRead(request.method, url.pathname);
          if (fixture) {
            observations.reads.push(`${request.method} ${url.pathname}`);
            response.writeHead(200, { "Content-Type": "application/json" });
            response.end(JSON.stringify(fixture));
            return;
          }
          rejectedBusinessRequests += 1;
          observations.rejected.push(`${request.method} ${url.pathname}`);
          response.writeHead(503, { "Content-Type": "application/json" });
          response.end(JSON.stringify({ error: "Native UI fixtures do not connect to business services" }));
          return;
        }
        if (url.pathname === "/qa/health") {
          response.writeHead(200, { "Content-Type": "application/json" });
          response.end(JSON.stringify({ kind: "nexus-native-ui-fixture", port, rejectedBusinessRequests, ...observations }));
          return;
        }
        if (url.pathname === "/app.html" && !appShell) {
          const route = new URL(url.searchParams.get("desktop_route") ?? "/launcher", url.origin);
          const query = new URLSearchParams({ theme: route.searchParams.get("theme") ?? "light",
            locale: route.searchParams.get("locale") ?? "en", section: route.searchParams.get("section") ?? "foundation",
            qa_case: route.searchParams.get("qa_case") ?? "initial" });
          response.writeHead(302, { Location: `/ui-gallery.html?${query}` });
          response.end();
          return;
        }
        next();
      });
    },
  }],
});
await server.listen();
console.log(`NEXUS_NATIVE_UI_SERVER ${port}`);
process.on("SIGTERM", async () => { closeSocket(); await server.close(); process.exit(0); });
