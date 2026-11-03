// INPUT: 同仓库开发 Gallery、真实 App 入口与完全拦截的本地接口夹具。
// OUTPUT: 滚动、认证、DM/Team 与设置等 jsdom 无法覆盖的真实浏览器流程回归。
// POS: 手动运行的浏览器流程测试入口；单一 Chromium 桌面项目，不承担视觉或像素验收。

import { defineConfig } from "@playwright/test";

const port = process.env.NEXUS_BROWSER_TEST_PORT ?? "3100";
const baseURL = `http://127.0.0.1:${port}`;

export default defineConfig({
  testDir: "./browser-tests",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  workers: 2,
  timeout: 30_000,
  expect: { timeout: 5_000 },
  reporter: [["list"], ["html", { open: "never" }]],
  outputDir: "test-results",
  use: {
    baseURL,
    headless: true,
    reducedMotion: "reduce",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  projects: [{
    name: "chromium-light-zh-1440",
    metadata: { theme: "light", locale: "zh" },
    use: { browserName: "chromium", viewport: { width: 1440, height: 900 } },
  }],
  webServer: {
    command: `npm run dev -- --mode browser-test --host 127.0.0.1 --port ${port}`,
    url: `${baseURL}/ui-gallery.html`,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
