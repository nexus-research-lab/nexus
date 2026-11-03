// INPUT: Production UI and real backend with a scripted local provider.
// OUTPUT: Desktop authentication rejection/success, streamed reply and history after reload.
// POS: End-to-end smoke; all Nexus HTTP/WebSocket traffic reaches the real server.
import { expect, test } from "@playwright/test";
test("desktop authentication, chat and persisted history", async ({ page, request, baseURL }) => {
  const token = process.env.NEXUS_SMOKE_TOKEN!;
  expect((await request.get("/nexus/v1/auth/status")).status()).toBe(401);
  const status = await request.get("/nexus/v1/auth/status", { headers: { "X-Nexus-Desktop-Token": token } });
  expect(status.ok()).toBe(true);
  expect((await status.json()).data.authenticated).toBe(true);
  const response = await request.get("/nexus/v1/rooms/dm/nexus", { headers: { "X-Nexus-Desktop-Token": token } });
  expect(response.ok()).toBe(true);
  const { data } = await response.json();
  await page.addInitScript(({ baseURL, token }) => {
    window.__NEXUS_DESKTOP_RUNTIME__ = { app_mode: "desktop", api_base_url: baseURL + "/nexus/v1", auth_token: token, ws_url: baseURL!.replace("http", "ws") + "/nexus/v1/chat/ws" };
    localStorage.setItem("nexus-locale", "en");
    localStorage.setItem("nexus:onboarding:tours", JSON.stringify({ "launcher-guide": true, "sidebar-navigation": true }));
    localStorage.setItem("nexus:onboarding:dismissed-tours", JSON.stringify({ "launcher-guide": true, "sidebar-navigation": true }));
  }, { baseURL, token });
  const entry = `/app.html?desktop_route=${encodeURIComponent(`/rooms/${data.room.id}/conversations/${data.conversation.id}`)}`;
  await page.goto(entry);
  const editor = page.locator("textarea").first();
  await expect(editor).toBeVisible();
  await editor.fill("Nexus smoke request: reply once without tools");
  await editor.press("Enter");
  const feed = page.locator("[data-conversation-round-id]");
  await expect(feed.getByText("Nexus real backend smoke reply", { exact: true })).toBeVisible({ timeout: 45_000 });
  await page.goto(entry);
  await expect(feed.getByText("Nexus real backend smoke reply", { exact: true })).toBeVisible();
  await expect(feed.getByText("Nexus smoke request: reply once without tools", { exact: true })).toBeVisible();
});
