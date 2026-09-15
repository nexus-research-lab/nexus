// INPUT: 真实 Team 页面、成员弹窗与隔离 Relay 响应。
// OUTPUT: 设置保存、危险动作确认、解散后的只读撤权和窄屏布局证据。
// POS: 浏览器 UI 合同；不接触线上账号、数据库或执行真实 Agent。
import { expect, test } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

test("online room settings and dissolution use the real dialog and revoke the composer", async ({page, context}, info) => {
  const zh = info.project.metadata.locale === "zh";
  const text = (cn: string, en: string) => zh ? cn : en;
  const now = "2026-09-14T00:00:00Z";
  const room = {id: "online-room", organization_id: "org", name: "Online QA", description: "", avatar: "", coordinator_agent_id: "agent",
    host_auto_reply_enabled: false, private_messages_enabled: false, skill_names: [], membership_version: 1, configuration_version: 1, created_at: now, updated_at: now};
  const details = {room, current_user_role: "owner", conversation: {id: "online-conversation", room_id: room.id, type: "room", high_water_message_seq: 0,
    last_activity_at: null, sync_stream_id: "online-stream", stream_epoch: "epoch", high_water_sync_event_seq: 0},
    members: [{room_id: room.id, member_type: "user", member_id: "ui-fixture", role: "owner", state: "active", invited_by_user_id: "ui-fixture", joined_at: now, created_at: now, updated_at: now}]};
  let dissolved = false;
  let nodeState = "disconnected";
  let executionEnabled = false;
  const writes: Record<string, unknown>[] = [];
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  await context.routeWebSocket("**/nexus/v1/chat/ws", (socket) => socket.onMessage((raw) => {
    if (JSON.parse(raw.toString()).type === "ping") socket.send(JSON.stringify({event_type: "pong"}));
  }));
  await context.routeWebSocket("**/nexus/v1/team/stream?*", () => {});
  await context.route("**/*", async (route) => {
    const request = route.request(); const url = new URL(request.url()); const path = url.pathname;
    if (path === "/nexus/v1/team-node") {
      if (request.method() === "POST") {
        const input = request.postDataJSON();
        expect(input).toEqual({name: "Nexus", agent_ids: ["agent"], enable_execution: nodeState === "authorized"});
        executionEnabled = input.enable_execution;
        nodeState = "authorized";
        return route.fulfill({json: {data: {ok: true}}});
      }
      if (request.method() === "DELETE") { nodeState = "revoked"; return route.fulfill({json: {data: {ok: true}}}); }
      return route.fulfill({json: {data: {state: nodeState, name: "Nexus", agent_ids: nodeState === "authorized" ? ["agent"] : [], candidates: [{id: "agent", name: "Research Agent"}], execution_available: true, execution_enabled: executionEnabled, jobs: executionEnabled ? [{id: "job", agent_id: "agent", state: "running", room_id: "local-room", conversation_id: "local-conversation"}] : []}}});
    }
    if (path === "/nexus/v1/auth/status") return route.fulfill({json: {data: {...appShellRead("GET", path)!.data as object, auth_method: "password", role: "owner", control_user_id: "ui-fixture", organization_id: "org"}}});
    if (path === "/nexus/v1/team/rooms") return route.fulfill({json: {data: {rooms: dissolved ? [] : [details]}}});
    if (path === "/nexus/v1/team/invitations") return route.fulfill({json: {data: {invitations: [], recovery_rooms: []}}});
    if (path === "/auth/v1/directory/agents") return route.fulfill({json: {data: [{agent_id: "agent", name: "Research Agent", owner_user_id: "ui-fixture"}]}});
    if (["/auth/v1/agents", "/auth/v1/directory/members"].includes(path)) return route.fulfill({json: {data: []}});
    if (path === `/nexus/v1/team/rooms/${room.id}`) {
      if (request.method() === "PATCH") {
        const input = request.postDataJSON(); writes.push(input);
        expect(input.expected_configuration_version).toBe(room.configuration_version);
        expect(request.headers()["idempotency-key"]).toBeTruthy();
        if (input.name) room.name = input.name;
        if (input.coordinator_agent_id !== undefined) room.coordinator_agent_id = input.coordinator_agent_id;
        dissolved = input.dissolve === true; room.configuration_version++;
        return route.fulfill({json: {data: {room_id: room.id, configuration_version: room.configuration_version, replayed: false}}});
      }
      if (dissolved) return route.fulfill({status: 404, json: {code: "resource_not_found", message: "gone"}});
      return route.fulfill({json: {data: details}});
    }
    if (path.endsWith("/snapshot")) return route.fulfill({json: {data: {conversation_id: "online-conversation", stream_id: "online-stream", stream_epoch: "epoch", snapshot_seq: 1, messages: [{id: "agent-result", conversation_id: "online-conversation", message_seq: 1, author_type: "agent", author_user_id: "ui-fixture", author_agent_id: "agent", author_display_name: "agent", author_username: "agent", delivery_id: "delivery", output_kind: "final", client_message_id: "output", content: {version: 1, blocks: [{type: "markdown", text: "**Completed result**"}]}, created_at: now}], has_more: false, through_message_seq: 1, next_message_seq: 1}}});
    const fixture = appShellRead(request.method(), path);
    if (fixture) return route.fulfill({json: fixture});
    if (!["localhost", "127.0.0.1"].includes(url.hostname) || request.method() !== "GET") return route.abort();
    if (["fetch", "xhr"].includes(request.resourceType())) return route.fulfill({json: {data: []}});
    return route.continue();
  });
  const params = new URLSearchParams({room_id: room.id, theme: String(info.project.metadata.theme), locale: String(info.project.metadata.locale)});
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/team?${params}`)}`);
  await expect(page.getByText("Research Agent", {exact: true})).toBeVisible();
  await expect(page.getByText("Completed result", {exact: true})).toBeVisible();
  await expect(page.getByText("Agent", {exact: true})).toBeVisible();
  await page.getByRole("button", {name: text("本机授权", "Host authorization"), exact: true}).click();
  const node = page.getByRole("dialog", {name: text("本机授权", "Host authorization"), exact: true});
  await expect(node).toBeVisible();
  expect(await node.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await node.getByRole("checkbox", {name: "Research Agent"}).check();
  await node.getByRole("button", {name: text("授权所选 Agent", "Authorize selected Agents"), exact: true}).click();
  await expect(node.getByText(text("设备授权已登记", "Host authorization registered"), {exact: true})).toBeVisible();
  expect(executionEnabled).toBe(false);
  await node.getByRole("button", {name: text("开启任务执行", "Enable task execution"), exact: true}).click();
  await expect(node.getByRole("link", {name: text("打开执行会话", "Open execution conversation")})).toHaveAttribute("href", "/rooms/local-room/conversations/local-conversation");
  expect(executionEnabled).toBe(true);
  expect(await node.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await node.getByRole("button", {name: text("撤销授权", "Revoke authorization"), exact: true}).click();
  await expect(node.getByText(text("设备授权已撤销", "Host authorization revoked"), {exact: true})).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(node).toHaveCount(0);
  await page.getByRole("button", {name: text("成员", "Members"), exact: true}).click();
  const dialog = page.getByRole("dialog", {name: text("群成员", "Room members"), exact: true});
  await expect(dialog).toBeVisible();
  expect(await dialog.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await dialog.getByRole("textbox", {name: text("群聊名称", "Room name")}).fill("Renamed online room");
  await dialog.getByRole("button", {name: text("保存", "Save"), exact: true}).click();
  await expect.poll(() => room.name).toBe("Renamed online room");
  await dialog.getByRole("button", {name: text("取消主持 Agent", "Clear coordinator")}).click();
  await expect.poll(() => room.coordinator_agent_id).toBe("");
  await dialog.getByRole("button", {name: text("解散群聊", "Dissolve room"), exact: true}).click();
  expect(dissolved).toBe(false);
  const confirm = page.getByRole("dialog", {name: text("解散群聊", "Dissolve room"), exact: true});
  await confirm.getByRole("button", {name: text("解散群聊", "Dissolve room"), exact: true}).click();
  await expect(page.getByRole("textbox", {name: text("团队消息", "Team message")})).toBeDisabled();
  expect(dissolved).toBe(true);
  expect(writes).toHaveLength(3);
  expect(errors).toEqual([]);
});
