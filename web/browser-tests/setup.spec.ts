// INPUT: Real /setup route with isolated local credentials and fully intercepted Control APIs.
// OUTPUT: Shared access layout, reachable fields, exact submission and read-only failure reconciliation evidence.
// POS: First-run setup browser regression; no request may reach a real deployment or create an account.

import { expect, test } from "@playwright/test";
import { moveKeyboardFocus } from "./keyboard";

test("setup shares access presentation and preserves validation, pending state and reconciliation", async ({ page }, info) => {
  const { theme, locale } = info.project.metadata;
  const copy = (zh: string, en: string) => locale === "zh" ? zh : en;
  const unexpectedRequests: string[] = [];
  const errors: string[] = [];
  const submissions: Array<{ body: unknown; authorization: string | undefined }> = [];
  let setupEnabled = true;
  let statusReads = 0;
  let releaseSetup = () => {};
  const setupResponse = new Promise<void>((resolve) => { releaseSetup = resolve; });
  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(({ theme, locale }) => {
    localStorage.setItem("nexus-theme", theme);
    localStorage.setItem("nexus-locale", locale);
  }, { theme, locale });
  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    if (request.url() === "https://fontsapi.zeoseven.com/309/main/result.css" && method === "GET") {
      await route.abort();
      return;
    }
    if (url.pathname === "/nexus/v1/runtime/options" && method === "GET") {
      await route.fulfill({ json: { data: { default_agent_id: "setup-browser-fixture" } } });
      return;
    }
    if (url.pathname === "/nexus/v1/auth/status" && method === "GET") {
      statusReads += 1;
      await route.fulfill({ json: { data: { auth_required: true, authenticated: false, password_login_enabled: true,
        setup_enabled: setupEnabled, setup_required: true, username: null } } });
      return;
    }
    if (url.pathname === "/auth/v1/setup" && method === "POST") {
      submissions.push({ body: request.postDataJSON(), authorization: request.headers().authorization });
      await setupResponse;
      await route.fulfill({ status: 503, json: { error: "Fixture unavailable" } });
      return;
    }
    if (["fetch", "xhr"].includes(request.resourceType()) || !["127.0.0.1", "localhost"].includes(url.hostname) || method !== "GET") {
      unexpectedRequests.push(`${method} ${url.pathname}`);
      await route.abort();
      return;
    }
    await route.continue();
  });
  await page.goto("/setup");
  const panel = page.getByRole("region", { name: copy("创建首个 owner", "Create the first owner"), exact: true });
  await expect(panel).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
  await expect(page.locator("html")).toHaveAttribute("lang", locale === "zh" ? "zh-CN" : "en");
  await page.evaluate(() => document.fonts.ready);
  const introduction = page.locator("[data-access-introduction]");
  const heading = introduction.getByRole("heading", { level: 1 });
  expect(await heading.evaluate((element) => getComputedStyle(element).fontSize)).toBe(page.viewportSize()!.width < 640 ? "44px" : "64px");
  await expect.poll(() => page.locator("main").evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  expect(await panel.evaluate((element) => getComputedStyle(element).borderRadius)).toBe("16px");
  expect(await panel.evaluate((element) => getComputedStyle(element).backgroundColor)).not.toBe("rgba(0, 0, 0, 0)");
  await info.attach("setup-introduction", { body: await introduction.screenshot({ animations: "disabled" }), contentType: "image/png" });

  const capability = panel.getByLabel(/^Setup code/);
  const password = panel.getByLabel(new RegExp(`^${copy("登录密码", "Password")}`));
  const confirm = panel.getByLabel(new RegExp(`^${copy("确认密码", "Confirm password")}`));
  const submit = panel.getByRole("button", { name: copy("建立 Nexus", "Create Nexus"), exact: true });
  await expect(submit).toBeDisabled();
  await expect(capability).toHaveAttribute("type", "password");
  await expect(capability).toHaveAttribute("autocomplete", "off");
  await capability.focus();
  await page.keyboard.insertText("fixture-setup-capability-0123456789");
  await moveKeyboardFocus(page, info);
  await expect(panel.getByLabel(new RegExp(`^${copy("部署名称", "Deployment name")}`))).toBeFocused();
  await password.fill("fixture-password");
  await confirm.fill("different-password");
  await expect(submit).toBeDisabled();
  await confirm.fill("fixture-password");
  await moveKeyboardFocus(page, info);
  await expect(submit).toBeFocused();
  await expect(submit).toBeEnabled();
  await expect.poll(() => panel.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await info.attach("setup-form", { body: await panel.screenshot({ animations: "disabled" }), contentType: "image/png" });
  const beforeSubmitReads = statusReads;
  try {
    await page.keyboard.press("Enter");
    await expect.poll(() => submissions.length).toBe(1);
    const pending = panel.getByRole("button", { name: copy("正在建立...", "Creating..."), exact: true });
    await expect(pending).toBeDisabled();
    await expect(pending).toHaveAttribute("aria-busy", "true");
  } finally {
    releaseSetup();
  }
  await expect(panel.getByRole("alert")).toContainText(copy("初始化结果尚未确认", "Setup result is not confirmed yet"));
  await expect.poll(() => statusReads).toBeGreaterThan(beforeSubmitReads);
  expect(submissions).toEqual([{ authorization: "Bearer fixture-setup-capability-0123456789", body: {
    deployment_name: "Nexus", username: "admin", display_name: "Admin", password: "fixture-password",
  } }]);
  expect(await page.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }))).not.toMatch(/fixture-password|fixture-setup-capability/);
  setupEnabled = false;
  await page.reload();
  await expect(page.getByRole("heading", { name: copy("首次初始化尚未开放", "First-run setup is not enabled"), exact: true })).toBeVisible();
  await expect(page.locator("input")).toHaveCount(0);
  expect(submissions).toHaveLength(1);
  expect(unexpectedRequests).toEqual([]);
  expect(errors).toEqual([]);
});
