// INPUT: 真实运营入口、隔离的成员/订阅/项目/Provider 快照与主题视口矩阵。
// OUTPUT: 五个子页的内容层级、展开表单、控件尺寸和无横向溢出的证据。
// POS: 运营浏览器回归；所有 API 由本地样例响应，禁止请求真实写接口。
import { expect, test, type Locator } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

const plan = { plan_key: "team", display_name: "Team Research · 团队研究套餐", status: "active", monthly_token_limit: 1_000_000, notes: "", sort_order: 1 };
const member = { user_id: "qa-member", deployment_id: "qa", username: "research-team", display_name: "Research and Operations · 研究与运营团队", role: "member", membership_status: "active", created_at: "2026-09-01", updated_at: "2026-09-01" };
const provider = { id: "qa-provider", provider: "qa-provider", preset_key: "custom", visibility: "public", provider_kind: "llm", api_format: "responses", display_name: "Research Provider · 公共研究模型服务", base_url: "https://api.example.test/v1", models_path: "/models", enabled: true, can_manage: true, configuration_version: 1, auth_token_masked: "sk-***demo", usage_count: 0, used_by_agents: [], last_test_status: "", last_test_error: "", agent_runtime_supported: true, models: [] };
const reads = new Map<string, unknown>([
  ["/auth/v1/members", [member, { ...member, user_id: "qa-suspended", username: "suspended", display_name: "Suspended member", membership_status: "revoked" }]],
  ["/auth/v1/subscription/overview", { plans: [plan], accounts: [{ ...member, user_status: "active", plan_key: "team", plan_name: plan.display_name, monthly_token_limit: plan.monthly_token_limit }], updated_at: "2026-09-09" }],
  ["/nexus/v1/admin/subscription/usage", { accounts: [{ control_user_id: member.user_id, used_tokens: 240_000, session_count: 18, message_count: 120 }], period_start: "2026-09-01", period_end: "2026-10-01", updated_at: "2026-09-09" }],
  ["/nexus/v1/projects", [{ project_id: "research-team-with-a-long-project-name", root: "/workspace/projects/research-team", generation: 7, group_name: "qa", gid: 1, members: { "research-member-with-a-long-identifier": "write" } }]],
  ["/nexus/v1/admin/subscription/providers", [provider]],
  ["/nexus/v1/settings/provider-presets", [{ preset_key: "custom", provider_kind: "llm", endpoint_mode: "custom", display_name: "Custom", description: "", key_url: "", default_api_format: "responses", formats: [{ api_format: "responses", base_url: "", models_path: "/models" }] }]],
  ["/nexus/v1/settings/preferences", {}],
]);

async function expectContained(surface: Locator) {
  expect(await surface.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  for (const field of await surface.locator('input:not([type=checkbox]):not([role=searchbox]):visible, select:visible, button[aria-haspopup=listbox]:visible').all()) {
    const box = await field.boundingBox();
    expect(await field.evaluate((element) => {
      const label = element.closest("label");
      return !label || element.getBoundingClientRect().right <= label.getBoundingClientRect().right + 1;
    })).toBe(true);
    expect(box?.height).toBeGreaterThanOrEqual(32);
    expect(box?.width).toBeGreaterThan(60);
  }
}

for (const aclEnabled of [true, false]) {
test(`operations subpages keep clear hierarchy and aligned responsive controls (ACL ${aclEnabled ? "enabled" : "disabled"})`, async ({ page, context }, info) => {
  const zh = info.project.metadata.locale === "zh";
  const text = (cn: string, en: string) => zh ? cn : en;
  let projectRequests = 0;
  const rejected: string[] = [];
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  await context.addInitScript(() => localStorage.setItem("nexus:onboarding:tours", JSON.stringify({ "sidebar-navigation": true })));
  await context.routeWebSocket("**/nexus/v1/chat/ws", (socket) => socket.onMessage((raw) => {
    if (JSON.parse(raw.toString()).type === "ping") socket.send(JSON.stringify({ event_type: "pong" }));
  }));
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.pathname === "/nexus/v1/projects") projectRequests++;
    if (url.pathname === "/nexus/v1/runtime/options") return route.fulfill({ json: { data: { default_agent_id: "qa-main", project_permissions_enabled: aclEnabled } } });
    if (url.pathname === "/nexus/v1/auth/status") {
      return route.fulfill({ json: { data: { ...(appShellRead("GET", url.pathname)!.data as Record<string, unknown>), auth_method: "password", role: "owner" } } });
    }
    if (request.method() === "GET" && reads.has(url.pathname)) return route.fulfill({ json: { data: reads.get(url.pathname) } });
    const fixture = appShellRead(request.method(), url.pathname);
    if (fixture) return route.fulfill({ json: fixture });
    if (!["localhost", "127.0.0.1"].includes(url.hostname)) return route.abort();
    if (["fetch", "xhr"].includes(request.resourceType()) || request.method() !== "GET") {
      rejected.push(`${request.method()} ${url.pathname}`);
      return route.abort();
    }
    return route.continue();
  });
  const params = new URLSearchParams({ section: aclEnabled ? "operations-members" : "operations-projects", theme: String(info.project.metadata.theme), locale: String(info.project.metadata.locale) });
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/settings?${params}`)}`);
  if (!aclEnabled) {
    await expect(page).toHaveURL(/\/settings$/);
    const menu = page.getByRole("button", { name: text("设置导航", "Settings navigation"), exact: true });
    if (await menu.isVisible()) await menu.click();
    await expect(page.getByRole("navigation").getByRole("button", { name: text("项目权限", "Project access"), exact: true })).toHaveCount(0);
    await expect(page.locator("[data-operations-page]")).toHaveCount(0);
    expect(projectRequests).toBe(0);
    return;
  }
  const surface = page.locator("[data-operations-page]");
  const selectPage = async (section: string, cn: string, en: string) => {
    const menu = page.getByRole("button", { name: text("设置导航", "Settings navigation"), exact: true });
    if (await menu.isVisible()) await menu.click();
    const nav = page.getByRole("navigation", { name: text("设置", "Settings"), exact: true });
    await expect(nav.getByRole("button", { name: text(cn, en), exact: true })).toHaveCSS("font-weight", "400");
    await nav.getByRole("button", { name: text(cn, en), exact: true }).click();
    await expect(surface).toHaveAttribute("data-operations-page", section);
    await expect(page).toHaveURL(new RegExp(`section=${section}`));
    await expect(page.getByRole("dialog")).toHaveCount(0);
  };
  await expect(page.getByRole("group", { name: text("运营", "Operations"), exact: true })).toHaveCount(0);
  await expect(surface.getByText(member.display_name, { exact: true })).toBeVisible();
  await info.attach("operations-navigation", { body: await page.screenshot({ path: info.outputPath("operations-navigation.png") }), contentType: "image/png" });
  const navigationButton = page.getByRole("button", { name: text("设置导航", "Settings navigation"), exact: true });
  if (await navigationButton.isVisible()) {
    await navigationButton.focus();
    await navigationButton.press("Enter");
    await info.attach("operations-navigation-drawer", { body: await page.screenshot({ path: info.outputPath("operations-navigation-drawer.png") }), contentType: "image/png" });
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(navigationButton).toBeFocused();
  }

  await expect(surface.locator("input").first()).not.toBeVisible();
  await surface.locator("summary").first().click();
  await expect(surface.locator("#member-username")).toBeVisible();
  const role = surface.locator("#member-role");
  await expect(role).toHaveAttribute("aria-haspopup", "listbox");
  await role.click();
  await expect(page.getByRole("listbox").getByRole("option")).toHaveCount(3);
  await page.getByRole("option", { name: text("管理员", "Admin"), exact: true }).click();
  await expect(role).toContainText(text("管理员", "Admin"));
  await role.press("ArrowUp");
  await expect(role).toContainText(text("成员", "Member"));
  await role.press("Escape");
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await expectContained(surface);
  await info.attach("operations-members", { body: await surface.screenshot({ path: info.outputPath("operations-members.png") }), contentType: "image/png" });

  await selectPage("operations-subscriptions", "用户订阅", "User subscriptions");
  await expect(surface.getByText(member.display_name, { exact: true })).toBeVisible();
  await expect(surface.getByText("24%", { exact: false })).toBeVisible();
  await expectContained(surface);
  await info.attach("operations-subscriptions", { body: await surface.screenshot({ path: info.outputPath("operations-subscriptions.png") }), contentType: "image/png" });

  await selectPage("operations-plans", "套餐管理", "Plan management");
  const planDetails = surface.locator("details").filter({ hasText: plan.display_name });
  await expect(planDetails).toHaveCount(1);
  await expect(planDetails).not.toHaveAttribute("open", "");
  await planDetails.locator("summary").click();
  await expect(planDetails.locator("input").first()).toHaveValue(plan.display_name);
  await expectContained(surface);
  await info.attach("operations-plans", { body: await surface.screenshot({ path: info.outputPath("operations-plans.png") }), contentType: "image/png" });

  await selectPage("operations-providers", "订阅模型服务", "Subscription Provider");
  await expect(surface.getByText(provider.display_name, { exact: true }).first()).toBeVisible();
  await surface.getByRole("button", { name: provider.display_name, exact: true }).click();
  await expect(surface.locator('input[value="https://api.example.test/v1"]')).toBeVisible();
  await expectContained(surface);
  await info.attach("operations-providers", { body: await surface.screenshot({ path: info.outputPath("operations-providers.png") }), contentType: "image/png" });

  await selectPage("operations-projects", "项目权限", "Project access");
  await expect(surface.getByText("research-team-with-a-long-project-name", { exact: true })).toBeVisible();
  await expect(surface.getByText(text("权限版本", "Generation"), { exact: false })).toHaveCount(0);
  await surface.locator("summary").first().click();
  await expectContained(surface);
  await info.attach("operations-projects", { body: await surface.screenshot({ path: info.outputPath("operations-projects.png") }), contentType: "image/png" });
  await page.goBack();
  await expect(surface).toHaveAttribute("data-operations-page", "operations-providers");
  await page.reload();
  await expect(surface).toHaveAttribute("data-operations-page", "operations-providers");
  expect(errors).toEqual([]);
  expect(rejected).toEqual([]);
});

}
