import path from "node:path";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

const LIGHT_DESKTOP_ENTRY_HTML = new Set([
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

function isLightDesktopEntry(hostId: string): boolean {
  const hostFile = path.basename(hostId);
  return LIGHT_DESKTOP_ENTRY_HTML.has(hostFile);
}

function shouldPreloadForLightDesktopEntry(dep: string): boolean {
  const depFile = path.basename(dep);
  return LIGHT_DESKTOP_PRELOAD_PREFIXES.some((prefix) => depFile.startsWith(prefix));
}

function getNodePackageName(id: string): string | null {
  const normalizedId = id.split(path.sep).join("/");
  const nodeModulesParts = normalizedId.split("/node_modules/");
  const packagePath = nodeModulesParts[nodeModulesParts.length - 1];
  if (!packagePath) {
    return null;
  }

  const packageParts = packagePath.split("/");
  if (packageParts[0]?.startsWith("@")) {
    return packageParts[1] ? `${packageParts[0]}/${packageParts[1]}` : packageParts[0];
  }
  return packageParts[0] ?? null;
}

function resolveDevTarget(mode: string, key: string, fallbackPort: string): string {
  const environment = loadEnv(mode, process.cwd(), "VITE_");
  const port = Number.parseInt(environment[key] || fallbackPort, 10);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`Invalid ${key}: ${environment[key]}`);
  }
  return `http://127.0.0.1:${port}`;
}

export default defineConfig(({ mode }) => ({
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
        if (context.hostType !== "html" || !isLightDesktopEntry(context.hostId)) {
          return deps;
        }
        return deps.filter(shouldPreloadForLightDesktopEntry);
      },
    },
    rollupOptions: {
      input: {
        index: path.resolve(__dirname, "index.html"),
        app: path.resolve(__dirname, "app.html"),
        settings: path.resolve(__dirname, "settings.html"),
        oauth_callback: path.resolve(__dirname, "oauth-callback.html"),
      },
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return undefined;
          }
          const packageName = getNodePackageName(id);

          if (
            packageName === "react" ||
            packageName === "react-dom" ||
            packageName === "scheduler"
          ) {
            return "vendor-react";
          }

          if (
            packageName === "lucide-react" ||
            packageName === "framer-motion" ||
            packageName === "matter-js"
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
    strictPort: true,
    proxy: {
      "/nexus/v1": {
        target: resolveDevTarget(mode, "VITE_BACKEND_PORT", "8010"),
        changeOrigin: true,
        ws: true,
      },
      "/auth/v1": {
        target: resolveDevTarget(mode, "VITE_CONTROL_PORT", "8020"),
        changeOrigin: false,
      },
    },
  },
  preview: {
    host: "0.0.0.0",
    port: 3000,
    strictPort: true,
  },
}));
