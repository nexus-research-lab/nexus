// INPUT: src 下共置的组件测试与 @ 路径别名。
// OUTPUT: 在 jsdom 中运行 React 组件行为测试的独立 Vitest 配置。
// POS: 前端组件测试入口；Node 合同测试继续由 scripts/*.test.mjs 负责。

import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./src/test/setup.ts"],
  },
});
