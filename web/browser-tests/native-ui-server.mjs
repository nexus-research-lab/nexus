// INPUT: A private local port and native launcher routes from the UI harness.
// OUTPUT: Current-source Gallery pages with all business transport rejected.
// POS: Native UI fixture server; never participates in product builds or backend requests.

import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const port = Number(process.env.NEXUS_UI_TEST_PORT);
let rejectedBusinessRequests = 0;
if (!Number.isInteger(port) || port < 1024 || port > 65535 || port === 34343) throw new Error("An isolated test port is required");
const server = await createServer({
  root, mode: "native-ui-test", cacheDir: path.join(root, "node_modules/.vite-native-ui-test"),
  server: { host: "127.0.0.1", port, strictPort: true },
  plugins: [{
    name: "native-ui-fixtures",
    configureServer(vite) {
      vite.middlewares.use((request, response, next) => {
        const url = new URL(request.url ?? "/", `http://127.0.0.1:${port}`);
        if (url.pathname.startsWith("/nexus/") || url.pathname.startsWith("/auth/")) {
          rejectedBusinessRequests += 1;
          response.writeHead(503, { "Content-Type": "application/json" });
          response.end(JSON.stringify({ error: "Native UI fixtures do not connect to business services" }));
          return;
        }
        if (url.pathname === "/qa/health") {
          response.writeHead(200, { "Content-Type": "application/json" });
          response.end(JSON.stringify({ kind: "nexus-native-ui-fixture", port, rejectedBusinessRequests }));
          return;
        }
        if (url.pathname === "/app.html") {
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
process.on("SIGTERM", async () => { await server.close(); process.exit(0); });
