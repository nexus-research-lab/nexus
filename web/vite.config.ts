import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return undefined;
          }

          if (
            id.includes("react-markdown") ||
            id.includes("remark-") ||
            id.includes("rehype-") ||
            id.includes("/katex/")
          ) {
            return "vendor-markdown";
          }

          if (
            id.includes("lottie-react") ||
            id.includes("@lottiefiles/dotlottie-react") ||
            id.includes("lottie-web")
          ) {
            return "vendor-lottie";
          }

          if (
            id.includes("/react/") ||
            id.includes("/react-dom/") ||
            id.includes("scheduler")
          ) {
            return "vendor-react";
          }

          if (
            id.includes("lucide-react") ||
            id.includes("framer-motion") ||
            id.includes("matter-js")
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
