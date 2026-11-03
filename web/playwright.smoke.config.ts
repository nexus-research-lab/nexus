// INPUT: An isolated real backend started by TestBrowserSmoke.
// OUTPUT: Chromium authentication/chat/history smoke evidence without intercepted API responses.
// POS: Nightly/release smoke configuration; independent of mocked browser flow tests.
import { defineConfig } from "@playwright/test";
if (!process.env.NEXUS_SMOKE_BASE_URL) throw new Error("Run through TestBrowserSmoke");
export default defineConfig({
  testDir: "./smoke-tests", timeout: 60_000, workers: 1, retries: 0,
  use: { baseURL: process.env.NEXUS_SMOKE_BASE_URL, browserName: "chromium", viewport: { width: 1440, height: 900 }, trace: "retain-on-failure", screenshot: "only-on-failure" },
  outputDir: "test-results/smoke",
});
