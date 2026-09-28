// INPUT: 隔离的配对目录与 App 壳层读取夹具。
// OUTPUT: 配对身份、操作、等宽配置控件与技术详情的浏览器几何证据。
// POS: 配对页面回归；全部接口拦截，不访问真实账户。
import { expect, test } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

test("pairing controls align and technical details remain secondary", async ({ page, context }, info) => {
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  await context.routeWebSocket("**/nexus/v1/chat/ws", (socket) => socket.onMessage((raw) => {
    if (JSON.parse(raw.toString()).type === "ping") socket.send(JSON.stringify({ event_type: "pong" }));
  }));
  await context.route("**/*", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "GET" && path === "/nexus/v1/capability/pairings") return route.fulfill({ json: { data: [{
      pairing_id: "pair", agent_id: "qa-main", agent_name: "Nexus", channel_type: "wechat", chat_type: "dm",
      external_name: "Design Team", external_ref: "external-user-with-a-long-identity-1234567890", status: "active",
      source: "manual", created_at: "2026-09-01T10:00:00Z", updated_at: "2026-09-28T10:00:00Z", session_key: "private-session-key", binding_version: 1,
    }] } });
    if (request.method() === "GET" && path === "/nexus/v1/agents") return route.fulfill({ json: { data: [{
      agent_id: "qa-main", name: "Nexus", created_at: "2026-09-01", status: "idle", workspace_path: "/fixture", options: {},
    }] } });
    const room = { id: "room", room_type: "room", name: "Product team", private_messages_enabled: true };
    const members = [{ member_type: "agent", member_agent_id: "qa-main", participation_paused: false }];
    if (request.method() === "GET" && path === "/nexus/v1/rooms") return route.fulfill({ json: { data: [{ room, members }, { room: { ...room, id: "direct", room_type: "dm", private_messages_enabled: false, name: "Nexus" }, members }] } });
    if (request.method() === "GET" && path === "/nexus/v1/rooms/room/contexts") return route.fulfill({ json: { data: [{ room, members, sessions: [], member_agents: [], conversation: { id: "topic", title: "Login redesign" } }] } });
    if (request.method() === "GET" && path === "/nexus/v1/rooms/direct/contexts") return route.fulfill({ json: { data: [{ room: { ...room, id: "direct", room_type: "dm" }, members, sessions: [], member_agents: [], conversation: { id: "dm-topic", title: "Existing research" } }] } });
    const value = appShellRead(request.method(), path);
    if (value) return route.fulfill({ json: value });
    if (["fetch", "xhr"].includes(request.resourceType()) || request.method() !== "GET") return route.abort();
    return route.continue();
  });
  const params = new URLSearchParams({ theme: String(info.project.metadata.theme), locale: String(info.project.metadata.locale) });
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/capability/pairings?${params}`)}`);
  const card = page.locator("section").filter({ has: page.getByText("Design Team", { exact: true }) }).last();
  await expect(card).toBeVisible();
  const agent = card.locator("button[aria-haspopup='listbox']");
  await expect(agent).toContainText("Nexus");
  const target = card.locator("button[aria-haspopup='dialog']");
  await expect(target).toHaveAttribute("aria-expanded", "false");
  const a = (await agent.boundingBox())!;
  const b = (await target.boundingBox())!;
  expect(Math.abs(a.height - b.height)).toBeLessThanOrEqual(1);
  if (info.project.use.viewport!.width >= 768) {
    expect(Math.abs(a.y - b.y)).toBeLessThanOrEqual(1);
    expect(Math.abs(a.width - b.width)).toBeLessThanOrEqual(1);
  }
  expect(await card.evaluate((node) => node.scrollWidth - node.clientWidth)).toBeLessThanOrEqual(1);
  await expect(card.getByText("private-session-key", { exact: true })).not.toBeVisible();
  await card.locator("summary").click();
  await expect(card.getByText("private-session-key", { exact: true })).toBeVisible();
  const before = (await card.boundingBox())!;
  await target.click();
  const picker = page.getByRole("dialog");
  await expect(picker).toBeVisible();
  const pickerBefore = (await picker.boundingBox())!;
  await picker.getByRole("button", { name: "Product team", exact: true }).click();
  await picker.getByRole("button", { name: "Login redesign", exact: true }).click();
  await expect(picker.getByText(/Product team \/ Login redesign/)).toBeVisible();
  expect(await card.evaluate((node) => node.scrollWidth - node.clientWidth)).toBeLessThanOrEqual(1);
  expect(Math.abs((await card.boundingBox())!.height - before.height)).toBeLessThanOrEqual(1);
  expect(Math.abs((await picker.boundingBox())!.height - pickerBefore.height)).toBeLessThanOrEqual(1);
  await picker.getByRole("button", { name: /Nexus ·/ }).click();
  await picker.getByRole("button", { name: "Existing research" }).click();
  await expect(picker.getByText(/Nexus \/ Existing research/)).toBeVisible();
  expect(Math.abs((await picker.boundingBox())!.height - pickerBefore.height)).toBeLessThanOrEqual(1);
  await info.attach("picker", { body: await picker.screenshot({ path: info.outputPath("picker.png") }), contentType: "image/png" });
  await page.keyboard.press("Escape");
  await expect(picker).not.toBeVisible();
  await expect(target).toBeFocused();
  await info.attach("pairing-layout", { body: await card.screenshot({ path: info.outputPath("pairing-layout.png") }), contentType: "image/png" });
});
