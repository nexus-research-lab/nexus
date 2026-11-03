// INPUT: 真实运营入口与隔离的成员快照。
// OUTPUT: 部署用户创建与组织创建/解散流程的浏览器回归证据。
// POS: 运营浏览器回归；所有 API 由本地样例响应，禁止请求真实写接口。
import { expect, test, type Locator } from "@playwright/test";
import { appShellRead, APP_SHELL_INIT_SCRIPT } from "./native-ui-app-fixtures.mjs";

const member = { user_id: "qa-member", deployment_id: "qa", username: "research-team", display_name: "Research and Operations · 研究与运营团队", role: "member", membership_status: "active", created_at: "2026-09-01", updated_at: "2026-09-01" };

test("deployment owner creates an independent Web user without an organization", async ({ page, context }, info) => {
  const text = (cn: string, en: string) => info.project.metadata.locale === "zh" ? cn : en;
  const errors: string[] = [];
  const writes: Record<string, unknown>[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  await context.routeWebSocket("**/nexus/v1/chat/ws", (socket) => socket.onMessage((raw) => {
    if (JSON.parse(raw.toString()).type === "ping") socket.send(JSON.stringify({ event_type: "pong" }));
  }));
  await context.route("**/*", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path === "/nexus/v1/auth/status") return route.fulfill({ json: { data: {
      ...(appShellRead("GET", path)!.data as Record<string, unknown>),
      role: "owner", auth_method: "password", organization_id: "", organization_role: "",
    } } });
    if (path === "/auth/v1/deployment-members") {
      if (request.method() === "GET") return route.fulfill({ json: { data: [] } });
      const input = request.postDataJSON();
      writes.push(input);
      expect(input).toEqual({ username: "lijie", display_name: "Lijie", password: "test-password-123", role: "member" });
      return route.fulfill({ json: { data: { ...member, ...input, password: undefined, web_access_disabled: false } } });
    }
    const value = appShellRead(request.method(), path);
    if (value) return route.fulfill({ json: value });
    if (["fetch", "xhr"].includes(request.resourceType()) || request.method() !== "GET") return route.abort();
    return route.continue();
  });
  const params = new URLSearchParams({ section: "operations-members", theme: String(info.project.metadata.theme), locale: String(info.project.metadata.locale) });
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/settings?${params}`)}`);
  const surface = page.locator('[data-operations-page="operations-members"]');
  await expect(surface).toBeVisible();
  const trigger = surface.getByRole("button", { name: text("新建用户", "Create user"), exact: true });
  // WebKit 鼠标点击不聚焦按钮；用键盘打开以验证焦点归还。
  await trigger.focus();
  await page.keyboard.press("Enter");
  const dialog = page.getByRole("dialog", { name: text("新建用户", "Create user"), exact: true });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByLabel(new RegExp(`^${text("用户名", "Username")}`))).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
  await expect(trigger).toBeFocused();
  await page.keyboard.press("Enter");
  await dialog.getByLabel(new RegExp(`^${text("用户名", "Username")}`)).fill("lijie");
  await dialog.getByLabel(text("显示名称", "Display name"), { exact: true }).fill("Lijie");
  await dialog.getByLabel(new RegExp(`^${text("初始密码", "Initial password")}`)).fill("test-password-123");
  await dialog.getByLabel(new RegExp(`^${text("确认密码", "Confirm password")}`)).fill("test-password-123");
  await expectContained(dialog);
  await info.attach("deployment-user-create", { body: await page.screenshot({ path: info.outputPath("deployment-user-create.png") }), contentType: "image/png" });
  const submit = dialog.getByRole("button", { name: text("新建用户", "Create user"), exact: true });
  await submit.focus();
  await page.keyboard.press("Tab");
  await expect(dialog.getByRole("button", { name: text("关闭", "Close"), exact: true })).toBeFocused();
  await submit.focus();
  await page.keyboard.press("Enter");
  await expect(dialog).not.toBeVisible();
  await expect(trigger).toBeFocused();
  await expect(surface.getByText("@lijie", { exact: true })).toBeVisible();
  await expect(surface.getByText(text("可访问 Web", "Web enabled"), { exact: true })).toBeVisible();
  await expectContained(surface);
  await info.attach("deployment-user-directory", { body: await page.screenshot({ path: info.outputPath("deployment-user-directory.png") }), contentType: "image/png" });
  await trigger.click();
  await expect(dialog.getByLabel(new RegExp(`^${text("初始密码", "Initial password")}`))).toHaveValue("");
  expect(writes).toHaveLength(1);
  expect(errors).toEqual([]);
});

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

test("platform member creates and dissolves an organization without acquiring operations access", async ({ page, context }, info) => {
  const zh = info.project.metadata.locale === "zh";
  const text = (cn: string, en: string) => zh ? cn : en;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await context.addInitScript(APP_SHELL_INIT_SCRIPT);
  let organization: Record<string, unknown> = {};
  await context.route("**/*", async (route) => {
    const req = route.request();
    const path = new URL(req.url()).pathname;
    if (path === "/nexus/v1/auth/status") return route.fulfill({json:{data:{...(appShellRead("GET",path)!.data as Record<string,unknown>),role:"member",auth_method:"password",...organization}}});
    if (path === "/auth/v1/organization/create" && req.method() === "POST") {
      expect(req.postDataJSON()).toMatchObject({name:"My Research"});
      organization={organization_id:"created",organization_name:"My Research",organization_role:"owner"};
      return route.fulfill({json:{data:organization}});
    }
    if (path === "/auth/v1/organization/dissolve" && req.method() === "POST") { organization={}; return route.fulfill({json:{data:{}}}); }
    if (path === "/auth/v1/members") return route.fulfill({json:{data:[]}});
    const value=appShellRead(req.method(),path);
    if (value) return route.fulfill({json:value});
    if (["fetch","xhr"].includes(req.resourceType()) || req.method() !== "GET") return route.abort();
    return route.continue();
  });
  const params=new URLSearchParams({section:"operations-organization",theme:String(info.project.metadata.theme),locale:String(info.project.metadata.locale)});
  await page.goto(`/app.html?desktop_route=${encodeURIComponent(`/settings?${params}`)}`);
  await expect(page).toHaveURL(/section=operations-organization/);
  await page.getByRole("button",{name:text("创建组织","Create organization"),exact:true}).click();
  const dialog=page.getByRole("dialog");
  await dialog.getByRole("textbox",{name:text("组织名称","Organization name"),exact:true}).fill("My Research");
  await dialog.getByRole("button",{name:text("创建组织","Create organization"),exact:true}).click();
  await expect(page.getByText("My Research",{exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:text("邀请成员","Invite member"),exact:true})).toBeVisible();
  await expect(page.getByRole("button",{name:text("套餐管理","Plan management"),exact:true})).toHaveCount(0);
  await page.getByRole("button",{name:text("解散组织","Dissolve organization"),exact:true}).click();
  await expect(dialog).toContainText(text("此操作不可撤销","This cannot be undone"));
  await info.attach("organization-dissolve-confirm",{body:await page.screenshot({path:info.outputPath("organization-dissolve-confirm.png")}),contentType:"image/png"});
  await dialog.getByRole("button",{name:text("解散组织","Dissolve organization"),exact:true}).click();
  await expect(page.getByRole("button",{name:text("创建组织","Create organization"),exact:true})).toBeVisible();
  expect(errors).toEqual([]);
});
