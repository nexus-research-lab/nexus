// INPUT: 真实 /login 路由、固定主题/语言/视口与完全拦截的认证接口夹具。
// OUTPUT: 登录表单无横向溢出、键盘/提交阻塞与状态重读的浏览器回归证据。
// POS: 登录页浏览器 smoke；API 全拦截，远端聊天中文字体不加载，只验收本地 App sans。

import { expect, test, type Page, type TestInfo } from "@playwright/test";

function copy(info: TestInfo, zh: string, en: string): string {
  return info.project.metadata.locale === "zh" ? zh : en;
}

async function mockLoginRoute(page: Page, info: TestInfo, passwordLoginEnabled = true) {
  const { theme, locale } = info.project.metadata;
  const errors: string[] = [];
  const unexpectedRequests: string[] = [];
  const loginBodies: unknown[] = [];
  let statusReads = 0;
  let releaseLogin = () => {};
  const loginResponse = new Promise<void>((resolve) => { releaseLogin = resolve; });

  page.on("pageerror", (error) => errors.push(error.message));
  await page.addInitScript(({ theme, locale }) => {
    window.localStorage.setItem("nexus-theme", theme);
    window.localStorage.setItem("nexus-locale", locale);
  }, { theme, locale });

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const method = request.method();
    // globals.css imports the optional conversation prose font. Login uses the
    // local App sans stack, so keep this suite deterministic without fetching it.
    if (request.url() === "https://fontsapi.zeoseven.com/309/main/result.css"
      && method === "GET" && request.resourceType() === "stylesheet") {
      await route.abort();
      return;
    }
    if (url.pathname === "/nexus/v1/runtime/options" && method === "GET") {
      await route.fulfill({ json: { data: { default_agent_id: "login-browser-fixture" } } });
      return;
    }
    if (url.pathname === "/nexus/v1/auth/status" && method === "GET") {
      statusReads += 1;
      await route.fulfill({ json: { data: {
        auth_required: true,
        authenticated: false,
        password_login_enabled: passwordLoginEnabled,
        setup_required: false,
        username: null,
      } } });
      return;
    }
    if (url.pathname === "/auth/v1/login" && method === "POST") {
      loginBodies.push(request.postDataJSON());
      await loginResponse;
      await route.fulfill({ status: 503, json: { data: {
        failure: {
          version: 1,
          code: "login_result_unknown",
          category: "transport",
          effect: "unknown",
        },
      } } });
      return;
    }
    if (["fetch", "xhr"].includes(request.resourceType())
      || !["127.0.0.1", "localhost"].includes(url.hostname)
      || method !== "GET") {
      unexpectedRequests.push(`${method} ${url.pathname}`);
      await route.abort();
      return;
    }
    await route.continue();
  });

  await page.goto("/login");
  const panel = page.getByRole("region", { name: copy(info, "登录 Nexus", "Sign in to Nexus"), exact: true });
  await expect(panel).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
  await expect(page.locator("html")).toHaveAttribute("lang", locale === "zh" ? "zh-CN" : "en");
  await page.evaluate(() => document.fonts.ready);
  return { panel, errors, unexpectedRequests, loginBodies, releaseLogin, statusReads: () => statusReads };
}

test("login keeps credentials, keyboard order and exact submission blocking", async ({ page }, info) => {
  const fixture = await mockLoginRoute(page, info);
  const { panel, loginBodies } = fixture;
  const username = panel.getByRole("textbox", { name: copy(info, "用户名", "Username"), exact: true });
  const password = panel.getByLabel(copy(info, "密码", "Password"), { exact: true });
  const submit = panel.getByRole("button", { name: copy(info, "进入工作台", "Enter workspace"), exact: true });

  await expect.poll(() => panel.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  const bounds = await panel.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1);
  await expect(username).toHaveAttribute("autocomplete", "username");
  await expect(password).toHaveAttribute("autocomplete", "current-password");
  await expect(password).toHaveAttribute("type", "password");
  await username.focus();
  await page.keyboard.insertText("browser-fixture-user");
  await page.keyboard.press("Tab");
  await expect(password).toBeFocused();
  await page.keyboard.insertText("browser-fixture-password");
  await page.keyboard.press("Tab");
  await expect(submit).toBeFocused();
  expect(await submit.evaluate((element) => element.matches(":focus-visible"))).toBe(true);
  const beforeHover = await submit.boundingBox();
  await submit.hover();
  expect(await submit.boundingBox()).toEqual(beforeHover);
  await info.attach("login-keyboard-focus", {
    body: await panel.screenshot({ animations: "disabled" }),
    contentType: "image/png",
  });

  try {
    await page.keyboard.press("Enter");
    await expect.poll(() => loginBodies.length).toBe(1);
    expect(loginBodies[0]).toEqual({ username: "browser-fixture-user", password: "browser-fixture-password" });
    await expect(panel.getByRole("button", { name: copy(info, "登录中...", "Signing in..."), exact: true })).toBeDisabled();
  } finally {
    fixture.releaseLogin();
  }

  await expect(panel.getByText(copy(info, "无法确认是否已经登录", "Could not confirm whether sign-in completed"), { exact: true })).toBeVisible();
  await expect(submit).toBeDisabled();
  await expect(username).toHaveValue("browser-fixture-user");
  await expect(password).toHaveValue("browser-fixture-password");
  await password.focus();
  await page.keyboard.press("Enter");
  const previousReads = fixture.statusReads();
  await panel.getByRole("button", { name: copy(info, "检查登录", "Check sign-in"), exact: true }).click();
  await expect.poll(fixture.statusReads).toBeGreaterThan(previousReads);
  await expect(submit).toBeDisabled();
  expect(loginBodies).toHaveLength(1);
  await expect.poll(() => panel.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await info.attach("login-recovery", {
    body: await panel.screenshot({ animations: "disabled" }),
    contentType: "image/png",
  });
  expect(fixture.errors).toEqual([]);
  expect(fixture.unexpectedRequests).toEqual([]);
});

test("disabled password sign-in preserves the deployment explanation and safe refresh", async ({ page }, info) => {
  const fixture = await mockLoginRoute(page, info, false);
  const { panel } = fixture;
  await expect(panel.getByRole("heading", { name: copy(info, "当前实例未启用密码登录", "Password sign-in is disabled"), exact: true })).toBeVisible();
  await expect(panel.getByRole("textbox")).toHaveCount(0);
  await expect(panel.getByLabel(copy(info, "密码", "Password"), { exact: true })).toHaveCount(0);
  await expect.poll(() => panel.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  const previousReads = fixture.statusReads();
  await panel.getByRole("button", { name: copy(info, "检查登录", "Check sign-in"), exact: true }).click();
  await expect.poll(fixture.statusReads).toBeGreaterThan(previousReads);
  expect(fixture.loginBodies).toEqual([]);
  expect(fixture.errors).toEqual([]);
  expect(fixture.unexpectedRequests).toEqual([]);
});
