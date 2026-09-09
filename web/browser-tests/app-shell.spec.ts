// INPUT: Real App entry/Router, isolated read snapshots, theme/locale/viewport matrix.
// OUTPUT: Launcher/navigation/search geometry, local contact filtering and multi-page pin persistence.
// POS: App-shell browser regression; all HTTP/WS traffic stays in local fixtures.

import { expect, test } from "@playwright/test";
import { createRequire } from "node:module";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";
import { measureTextContrast } from "./color-contrast";

const localLottieWasm = createRequire(__filename).resolve("@lottiefiles/dotlottie-web/dotlottie-player.wasm");

test("real Launcher navigates to a readable responsive workbench and pins survive reload", async ({ page, context }, info) => {
  const errors: string[] = [];
  const rejected: string[] = [];
  const reads: string[] = [];
  const messages: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  context.on("page", (opened) => opened.on("pageerror", (error) => errors.push(error.message)));
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  await context.routeWebSocket("**/nexus/v1/chat/ws", (socket) => {
    socket.onMessage((raw) => {
      const { type } = JSON.parse(raw.toString()) as { type: string };
      messages.push(type);
      if (type === "ping") socket.send(JSON.stringify({ event_type: "pong" }));
      else if (!["subscribe_app_events", "unsubscribe_app_events"].includes(type)) rejected.push(`WS ${type}`);
    });
  });
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.href === "https://fontsapi.zeoseven.com/309/main/result.css") return route.abort();
    // Use the installed player's matching binary instead of its CDN fallback.
    if (["cdn.jsdelivr.net", "unpkg.com"].includes(url.hostname)
      && /^\/(?:npm\/)?@lottiefiles\/dotlottie-web@[^/]+\/dist\/dotlottie-player\.wasm$/.test(url.pathname)) {
      return route.fulfill({ path: localLottieWasm, contentType: "application/wasm" });
    }
    const fixture = appShellRead(request.method(), url.pathname);
    if (fixture) {
      reads.push(url.pathname);
      return route.fulfill({ json: fixture });
    }
    if (["localhost", "127.0.0.1"].includes(url.hostname) && request.method() === "GET"
      && /^\/lotties\/[^/]+\.lottie$/.test(url.pathname)) return route.continue();
    if (["fetch", "xhr"].includes(request.resourceType()) || request.method() !== "GET"
      || !["localhost", "127.0.0.1"].includes(url.hostname)) {
      rejected.push(`${request.method()} ${url.pathname}`);
      return route.abort();
    }
    return route.continue();
  });
  const params = new URLSearchParams({ theme: String(info.project.metadata.theme), locale: String(info.project.metadata.locale) });
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/launcher?${params}`)}`);
  const enter = page.locator('[data-tour-anchor="launcher-enter-app"]');
  const input = page.locator('[data-tour-anchor="launcher-composer"] input');
  await expect(enter).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", String(info.project.metadata.theme));
  await page.evaluate(() => document.fonts.ready);
  await input.fill("Inspect the local workspace");
  await expect(input).toHaveValue("Inspect the local workspace");
  await info.attach("app-launcher", { body: await page.screenshot(), contentType: "image/png" });
  await enter.click();
  await expect(page).toHaveURL(/\/app$/);
  const sidebar = page.locator(".sidebar-panel-shell");
  await expect(sidebar).toBeVisible();
  const labels = page.locator(".shell-navigation-rail button[aria-pressed] > span:nth-child(2)");
  await expect(labels).toHaveCount(3);
  const railWidth = await sidebar.locator(".shell-navigation-rail").evaluate((e) => e.getBoundingClientRect().width);
  const leadingPadding = await sidebar.evaluate((e) => parseFloat(getComputedStyle(e).paddingLeft));
  expect(railWidth).toBe(56 + leadingPadding);
  const railLeft = await sidebar.locator(".shell-navigation-rail").evaluate((e) => e.getBoundingClientRect().left);
  expect(railLeft).toBe(await sidebar.evaluate((e) => e.getBoundingClientRect().left));
  for (const label of await labels.all()) {
    const center = await label.evaluate((e) => {
      const box = e.getBoundingClientRect();
      return box.left + box.width / 2;
    });
    expect(center).toBeCloseTo(railLeft + railWidth / 2, 1);
    // scrollWidth rounds to integer pixels. Even subpixel clipping can show an
    // ellipsis, so measure the actual text against its content box instead.
    expect(await label.evaluate((e) => {
      const range = document.createRange();
      range.selectNodeContents(e);
      const text = range.getBoundingClientRect();
      const box = e.getBoundingClientRect();
      const style = getComputedStyle(e);
      return text.left >= box.left + parseFloat(style.paddingLeft) - 0.01
        && text.right <= box.right - parseFloat(style.paddingRight) + 0.01;
    })).toBe(true);
  }
  await info.attach("navigation-width", { body: JSON.stringify({ railWidth }), contentType: "application/json" });
  await expect.poll(() => page.locator("main").evaluate((e) => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await expect(page.locator(".desktop-app-stage")).toHaveCount(page.viewportSize()!.width <= 559 ? 0 : 1);
  await info.attach("app-workbench", { body: await page.screenshot(), contentType: "image/png" });
  await page.locator('[data-tour-anchor="sidebar-contacts-tab"]').click();
  const contacts = page.locator('[data-tour-anchor="sidebar-contacts-list"]');
  const isChinese = info.project.metadata.locale === "zh";
  const search = contacts.getByRole("searchbox", { name: isChinese ? "搜索联系人" : "Search contacts" });
  const create = contacts.getByRole("button", { name: isChinese ? "新建智能体" : "New Agent", exact: true });
  await expect(search).toHaveAttribute("placeholder", isChinese ? "搜索" : "Search");
  const searchGeometry = await search.evaluate((input) => {
    const style = getComputedStyle(input);
    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d")!;
    ctx.font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
    return {
      fontSize: parseFloat(style.fontSize),
      availableWidth: input.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight),
      placeholderWidth: ctx.measureText((input as HTMLInputElement).placeholder).width,
      height: input.parentElement!.getBoundingClientRect().height,
      radius: getComputedStyle(input.parentElement!).borderRadius,
    };
  });
  const actionGeometry = await create.evaluate((button) => ({
    height: button.getBoundingClientRect().height,
    width: button.getBoundingClientRect().width,
    radius: getComputedStyle(button).borderRadius,
  }));
  expect(searchGeometry.fontSize).toBe(14);
  expect(searchGeometry.placeholderWidth).toBeLessThanOrEqual(searchGeometry.availableWidth);
  const height = page.viewportSize()!.width <= 559 ? 48 : 36;
  expect(searchGeometry.height).toBe(height);
  expect(actionGeometry).toEqual({ height, width: height, radius: searchGeometry.radius });
  const contrast = await measureTextContrast(search, "::placeholder");
  expect(contrast.ratio).toBeGreaterThanOrEqual(4.5);
  await search.focus();
  await page.keyboard.press("Tab");
  await expect(create).toBeFocused();
  expect(await create.getAttribute("title")).toBeNull();
  await expect(page.getByRole("tooltip", { name: isChinese ? "新建智能体" : "New Agent" })).toBeVisible();
  await search.fill("Research");
  await expect(contacts.getByText("Research", { exact: true })).toBeVisible();
  await expect(contacts.getByText("Nexus", { exact: true })).toHaveCount(0);
  await contacts.getByRole("button", { name: isChinese ? "清除" : "Clear", exact: true }).click();
  await expect(search).toHaveValue("");
  await expect(search).toBeFocused();
  await expect(contacts.getByText("Nexus", { exact: true })).toBeVisible();
  await expect.poll(() => contacts.evaluate((e) => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await info.attach("sidebar-search-metrics", { body: JSON.stringify({ searchGeometry, actionGeometry, contrast }), contentType: "application/json" });
  await info.attach("app-contacts-search", { body: await page.screenshot(), contentType: "image/png" });
  await page.locator('[data-tour-anchor="sidebar-chat-tab"]').click();
  const sibling = await context.newPage();
  await sibling.goto("/app");
  await expect(sibling.locator(".sidebar-panel-shell")).toBeVisible();
  // Arrange through the real store action; the pin button adapter is covered by
  // its component test. All following storage and reload paths are real App code.
  await page.evaluate(async () => {
    const modulePath = "/src/store/room-navigation.ts";
    const { useRoomNavigationStore } = await import(modulePath);
    useRoomNavigationStore.getState().toggle_pinned_conversation({
      room_id: "qa-room", conversation_id: "qa-conversation", session_key: "qa-session", title: "QA pinned",
    });
  });
  await sibling.evaluate(async () => {
    const modulePath = "/src/store/room-navigation.ts";
    const { useRoomNavigationStore } = await import(modulePath);
    useRoomNavigationStore.getState().remember_last_active_conversation("qa-other-room", "qa-other-conversation");
  });
  await page.reload();
  const pinned = page.locator('[data-pinned-conversation-id="qa-conversation"]');
  await expect(pinned).toBeVisible();
  await expect(sibling.locator('[data-pinned-conversation-id="qa-conversation"]')).toBeVisible();
  await info.attach("app-pinned-after-refresh", { body: await page.screenshot(), contentType: "image/png" });
  await pinned.hover();
  await pinned.locator("[data-pinned-conversation-unpin]").click();
  await expect(sibling.locator('[data-pinned-conversation-id="qa-conversation"]')).toHaveCount(0);
  await page.reload();
  await expect(page.locator(".sidebar-panel-shell")).toBeVisible();
  await expect(pinned).toHaveCount(0);
  await sibling.close();
  await page.getByRole("link", { name: isChinese ? "回到 Launcher" : "Back to launcher", exact: true }).click();
  await expect(page).toHaveURL(/\/launcher$/);
  await expect(input).toHaveValue("");
  expect(reads).toContain("/nexus/v1/auth/status");
  expect(reads).toContain("/nexus/v1/launcher/bootstrap");
  expect(messages).toContain("subscribe_app_events");
  expect(errors).toEqual([]);
  expect(rejected).toEqual([]);
});
