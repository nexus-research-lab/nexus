// INPUT: 同仓库开发 Gallery 与固定浏览器、主题、语言、视口的成对覆盖矩阵。
// OUTPUT: 可本地和 CI 重复运行的浏览器几何、键盘、叠层测试及截图/失败 trace。
// POS: 共享 UI、认证与真实 App 壳层浏览器验收入口；独立 Vite 端口和依赖缓存，不依赖真实后端或登录凭据。

import { defineConfig } from "@playwright/test";

const themes = ["light", "dark", "rain"] as const;
const locales = ["zh", "en"] as const;
const chromiumWidths = [320, 767, 768, 1440];
const webkitWidths = [320, 1440];
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
  projects: [
    ...pairwiseProjects("chromium", chromiumWidths),
    ...pairwiseProjects("webkit", webkitWidths),
  ],
  webServer: {
    command: `npm run dev -- --mode browser-test --host 127.0.0.1 --port ${port}`,
    url: `${baseURL}/ui-gallery.html`,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});

// 主题 × 宽度全组合保留，语言按 (主题 + 宽度) 奇偶交替：任意两个维度的每种取值组合
// 都至少出现一次，用例数量约为完整笛卡尔积的一半；light-zh-1440 固定存在供 smoke 与截图使用。
function pairwiseProjects(browserName: "chromium" | "webkit", widths: readonly number[]) {
  return themes.flatMap((theme, themeIndex) => widths.map((width, widthIndex) => {
    const locale = locales[(themeIndex + widthIndex + 1) % locales.length];
    return {
      name: `${browserName === "webkit" ? "webkit-" : ""}${theme}-${locale}-${width}`,
      metadata: { theme, locale },
      use: {
        browserName,
        viewport: { width, height: width === 320 ? 640 : 900 },
      },
    };
  }));
}
