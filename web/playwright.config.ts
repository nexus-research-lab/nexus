// INPUT: 同仓库开发 Gallery 与固定浏览器、主题、语言、视口矩阵。
// OUTPUT: 可本地和 CI 重复运行的浏览器几何、键盘、叠层测试及截图/失败 trace。
// POS: 共享 UI 与登录页浏览器验收入口；独立 Vite 端口和依赖缓存，不依赖真实后端或登录凭据。

import { defineConfig } from "@playwright/test";

const themes = ["light", "dark", "rain"] as const;
const locales = ["zh", "en"] as const;
const widths = [320, 767, 768, 1440];
const baseURL = "http://127.0.0.1:3100";

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
  projects: [...themes.flatMap((theme) => locales.flatMap((locale) => widths.map((width) => ({
    name: `${theme}-${locale}-${width}`,
    metadata: { theme, locale },
    use: {
      browserName: "chromium" as const,
      viewport: { width, height: width === 320 ? 640 : 900 },
    },
  })))), ...themes.flatMap((theme) => locales.flatMap((locale) => [320, 1440].map((width) => ({
    name: `webkit-${theme}-${locale}-${width}`,
    metadata: { theme, locale },
    use: {
      browserName: "webkit" as const,
      viewport: { width, height: width === 320 ? 640 : 900 },
    },
  }))))],
  webServer: {
    command: "npm run dev -- --mode browser-test --host 127.0.0.1 --port 3100",
    url: `${baseURL}/ui-gallery.html`,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
