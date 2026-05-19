import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const LIGHT_DESKTOP_ENTRY_HTML = new Set([
  "launcher.html",
  "settings.html",
  "oauth-callback.html",
]);

const LIGHT_DESKTOP_PRELOAD_PREFIXES = [
  "auth-api-",
  "auth-context-",
  "desktop-entry-layout-",
  "desktop-entry-route-",
  "i18n-context-",
  "options-",
  "preload-helper-",
  "rolldown-runtime-",
  "root-bootstrap-",
  "route-paths-",
  "theme-context-",
  "tour-context-",
  "tour-provider-",
  "tour-state-",
  "utils-",
  "vendor-react-",
];

function is_light_desktop_entry(host_id: string): boolean {
  const host_file = path.basename(host_id);
  return LIGHT_DESKTOP_ENTRY_HTML.has(host_file);
}

function should_preload_for_light_desktop_entry(dep: string): boolean {
  const dep_file = path.basename(dep);
  return LIGHT_DESKTOP_PRELOAD_PREFIXES.some((prefix) => dep_file.startsWith(prefix));
}

function get_node_package_name(id: string): string | null {
  const normalized_id = id.split(path.sep).join("/");
  const node_modules_parts = normalized_id.split("/node_modules/");
  const package_path = node_modules_parts[node_modules_parts.length - 1];
  if (!package_path) {
    return null;
  }

  const package_parts = package_path.split("/");
  if (package_parts[0]?.startsWith("@")) {
    return package_parts[1] ? `${package_parts[0]}/${package_parts[1]}` : package_parts[0];
  }
  return package_parts[0] ?? null;
}

export default defineConfig({
  base: process.env.NEXUS_DESKTOP_BUILD === "1" ? "./" : "/",
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    modulePreload: {
      resolveDependencies(_, deps, context) {
        if (context.hostType !== "html" || !is_light_desktop_entry(context.hostId)) {
          return deps;
        }
        return deps.filter(should_preload_for_light_desktop_entry);
      },
    },
    rollupOptions: {
      input: {
        index: path.resolve(__dirname, "index.html"),
        app: path.resolve(__dirname, "app.html"),
        launcher: path.resolve(__dirname, "launcher.html"),
        settings: path.resolve(__dirname, "settings.html"),
        oauth_callback: path.resolve(__dirname, "oauth-callback.html"),
      },
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return undefined;
          }
          const package_name = get_node_package_name(id);

          if (
            package_name === "react" ||
            package_name === "react-dom" ||
            package_name === "scheduler"
          ) {
            return "vendor-react";
          }

          if (
            package_name === "lucide-react" ||
            package_name === "framer-motion" ||
            package_name === "matter-js"
          ) {
            return "vendor-ui";
          }

          return undefined;
        },
      },
    },
  },
  server: {
    host: "0.0.0.0",
    port: 3000,
    proxy: {
      "/nexus/v1": {
        target: "http://127.0.0.1:8010",
        changeOrigin: true,
        ws: true,
      },
    },
  },
  preview: {
    host: "0.0.0.0",
    port: 3000,
  },
});
