// INPUT: src 下共置的组件测试与 @ 路径别名。
// OUTPUT: 在 jsdom 中运行 React 组件行为测试的独立 Vitest 4 配置。
// POS: 前端组件测试入口；Node 合同测试继续由 scripts/*.test.mjs 负责。

import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

const maxWorkers = Number.parseInt(process.env.VITEST_MAX_WORKERS ?? "2", 10);

if (!Number.isInteger(maxWorkers) || maxWorkers < 1) {
  throw new Error(`VITEST_MAX_WORKERS must be a positive integer, got: ${process.env.VITEST_MAX_WORKERS}`);
}

export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    fs: {
      allow: [
        fileURLToPath(new URL(".", import.meta.url)),
        fileURLToPath(new URL("../docs", import.meta.url)),
      ],
    },
  },
  test: {
    maxWorkers,
    pool: "forks",
    // 由 jsdom 提供浏览器存储，避免 Node 25 的同名全局对象覆盖它。
    execArgv: ["--no-experimental-webstorage"],
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["./src/test/setup.ts"],
  },
});
