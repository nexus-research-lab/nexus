// INPUT: Real App entry/Router, isolated read snapshots, theme/locale/viewport matrix.
// OUTPUT: Launcher/navigation geometry and multi-page pin, reload and unpin evidence.
// POS: App-shell browser regression; all HTTP/WS traffic stays in local fixtures.

import { expect, test } from "@playwright/test";
import { createRequire } from "node:module";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

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
  expect(railWidth).toBe(64);
  for (const label of await labels.all()) {
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
  await page.getByRole("link").filter({ has: page.getByText("NEXUS", { exact: true }).first() }).click();
  await expect(page).toHaveURL(/\/launcher$/);
  await expect(input).toHaveValue("");
  expect(reads).toContain("/nexus/v1/auth/status");
  expect(reads).toContain("/nexus/v1/launcher/bootstrap");
  expect(messages).toContain("subscribe_app_events");
  expect(errors).toEqual([]);
  expect(rejected).toEqual([]);
});
