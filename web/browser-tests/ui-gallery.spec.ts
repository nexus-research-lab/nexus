// INPUT: 真实 shared UI Gallery、浏览器布局/焦点与主题语言视口矩阵。
// OUTPUT: 页面不溢出、模态焦点/滚动隔离、最上层 Escape、菜单定位及减弱动效的回归证据。
// POS: 浏览器行为门禁；截图用于人工复核，不把截图生成或 jsdom 当作像素比对结论。

import { expect, test, type Locator, type Page, type TestInfo } from "@playwright/test";

import { moveKeyboardFocus } from "./keyboard";
import { measureTextContrast } from "./color-contrast";

function copy(info: TestInfo, zh: string, en: string): string {
  return info.project.metadata.locale === "zh" ? zh : en;
}

async function openGallery(page: Page, info: TestInfo, section = "foundation") {
  const { theme, locale } = info.project.metadata;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  // The App chrome uses local system fonts; remote CJK prose is outside this fixture.
  await page.route("https://fontsapi.zeoseven.com/309/main/result.css", (route) => route.abort());
  await page.goto(`/ui-gallery.html?theme=${theme}&locale=${locale}&section=${section}`);
  const gallery = page.getByRole("main");
  await expect(gallery).toHaveAttribute("data-gallery-theme", theme);
  await expect(gallery).toHaveAttribute("data-gallery-locale", locale);
  await expect(page.getByRole("heading", { name: "Nexus UI Contract Gallery", exact: true })).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  return { gallery, errors };
}

async function expectInsideViewport(page: Page, surface: Locator) {
  await expect(surface).toBeVisible();
  await expect.poll(async () => {
    const bounds = await surface.boundingBox();
    const viewport = page.viewportSize()!;
    return Boolean(bounds && bounds.width > 0 && bounds.height > 0
      && bounds.x >= 0 && bounds.y >= 0
      && bounds.x + bounds.width <= viewport.width + 1
      && bounds.y + bounds.height <= viewport.height + 1);
  }).toBe(true);
}

async function capture(surface: Locator, info: TestInfo, name: string) {
  await info.attach(name, {
    body: await surface.screenshot({ animations: "disabled" }),
    contentType: "image/png",
  });
}

test("hover hints suppress repeated labels and reveal clipped text with the shared tooltip", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const redundant = page.locator("[data-gallery-redundant-tooltip]");
  await redundant.hover();
  await page.waitForTimeout(350);
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  await expect(redundant).not.toHaveAttribute("title");
  const clipped = page.locator("[data-gallery-truncated-tooltip]");
  await clipped.hover();
  await expect(page.getByRole("tooltip")).toHaveText("Long account name for tooltip verification");
  await expect(clipped).not.toHaveAttribute("title");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("source editors preserve text, keyboard focus and native read-only behavior", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const fixture = page.locator("[data-gallery-source-editor]");
  const source = fixture.getByRole("textbox", { name: copy(info, "源码草稿", "Source draft"), exact: true });
  const value = Array.from({ length: 24 }, (_, index) => `line ${index}  · 中文\tvalue`).join("\n");
  await source.fill(value);
  await expect(source).toHaveValue(value);
  await expect(source).toHaveCSS("font-size", "14px");
  await expect(source).toHaveCSS("line-height", "24px");
  expect(await source.evaluate((element) => element.scrollHeight > element.clientHeight)).toBe(true);
  await page.keyboard.press("ArrowLeft");
  expect(await source.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  await capture(source.locator(".."), info, "source-editor-focus");
  await page.keyboard.press("Tab");
  const saved = fixture.getByRole("textbox", { name: copy(info, "只读源码", "Read-only source"), exact: true });
  await expect(saved).toBeFocused();
  await page.keyboard.type("replacement");
  await expect(saved).toHaveValue("Saved  source\n\t只读内容");
  await expect(fixture.getByRole("textbox", { name: copy(info, "不可编辑的源码", "Disabled source"), exact: true })).toBeDisabled();
  await fixture.getByRole("button", { name: copy(info, "记录草稿", "Record draft"), exact: true }).click();
  await expect(fixture.locator("[data-gallery-source-record]")).toHaveText(JSON.stringify(value));
  await source.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, source);
  expect(errors).toEqual([]);
});

test("Memory directory selection and document actions retain file commands", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-memory]");
  const search = fixture.getByRole("searchbox");
  await search.fill("missing");
  await expect(fixture.getByText(copy(info, "没有匹配的记忆文件", "No matching memory files"), { exact: true })).toBeVisible();
  await fixture.getByRole("button", { name: copy(info, "清除筛选", "Clear filters"), exact: true }).click();
  const filter = fixture.getByRole("button", { name: copy(info, "筛选记忆类型", "Filter memory type"), exact: true });
  await filter.click();
  await page.getByRole("option", { name: copy(info, "引用", "Reference"), exact: true }).click();
  await expect(filter).toContainText(copy(info, "引用", "Reference"));
  await fixture.getByRole("button", { name: /跨区域项目资料与长期协作约定/ }).click();
  const header = fixture.locator(".nexus-memory-document-content").filter({ has: page.getByRole("heading", { level: 2 }) });
  await header.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, header);
  const title = header.getByRole("heading", { level: 2 });
  await expect(title).toContainText("跨区域项目资料与长期协作约定");
  await capture(header, info, "memory-long-title");
  await fixture.getByRole("button", { name: "Toggle runtime writing", exact: true }).click();
  await header.getByRole("button", { name: copy(info, "编辑", "Edit"), exact: true }).click();
  const save = header.getByRole("button", { name: copy(info, "保存", "Save"), exact: true });
  await expectInsideViewport(page, save);
  await save.click();
  await expect(fixture.locator("[data-gallery-memory-commands]")).toContainText('"save"');
  const back = header.getByRole("button", { name: copy(info, "返回记忆目录", "Back to memory list"), exact: true });
  if (await back.isVisible()) await back.click();
  await fixture.getByRole("button", { name: "Toggle empty memory", exact: true }).click();
  await expect(fixture.locator('[data-resource-state="empty"]')).toHaveCount(1);
  expect(errors).toEqual([]);
});

test("contact directory shares search chrome and keeps pending friend additions reviewable", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-contact-communication]");
  const search = fixture.getByRole("searchbox", { name: copy(info, "搜索联系人", "Search contacts"), exact: true });
  await search.fill("missing");
  await expect(fixture.getByText(copy(info, "没有匹配的联系人", "No matching contacts"), { exact: true })).toBeVisible();
  await fixture.getByRole("button", { name: copy(info, "清除", "Clear"), exact: true }).first().click();
  await expect(fixture.getByRole("button", { name: "Operations partner · 跨区域运营协作伙伴", exact: true })).toBeVisible();
  await fixture.getByRole("button", { name: copy(info, "添加好友", "Add friend"), exact: true }).click();
  const dialog = page.getByRole("dialog", { name: copy(info, "添加好友", "Add friend"), exact: true });
  await expectInsideViewport(page, dialog.locator(".dialog-shell"));
  const candidates = dialog.getByRole("searchbox", { name: copy(info, "搜索可添加的 Agent", "Search Agents to add"), exact: true });
  await expect(candidates).toBeFocused();
  await dialog.getByRole("button", { name: "Writer · 写作者", exact: true }).click();
  const note = dialog.getByRole("textbox", { name: copy(info, "备注", "Note"), exact: true });
  await note.fill("Research partner");
  await candidates.fill("missing");
  await expect(dialog.getByText(copy(info, "没有匹配的 Agent", "No matching Agents"), { exact: true })).toBeVisible();
  await expect(note).toHaveAccessibleDescription(/Writer/);
  const submit = dialog.getByRole("button", { name: copy(info, "添加好友", "Add friend"), exact: true });
  await submit.click();
  await expect(submit).toBeDisabled();
  await expect(candidates).toBeDisabled();
  await expect(note).toBeDisabled();
  for (const button of await dialog.getByRole("button").all()) await expect(button).toBeDisabled();
  await page.keyboard.press("Escape");
  await page.mouse.click(1, 1);
  await expect(dialog).toBeVisible();
  await expect(fixture.locator("[data-gallery-communication-commands]")).toHaveText('["add:writer:Research partner"]');
  await capture(dialog.locator(".dialog-shell"), info, "contact-add-pending");
  await page.evaluate(() => window.dispatchEvent(new Event("nexus-gallery-contact-add-result")));
  await expect(submit).toBeEnabled();
  await expect(note).toHaveValue("Research partner");
  await dialog.getByRole("button", { name: copy(info, "取消", "Cancel"), exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await fixture.getByRole("button", { name: "Toggle stale contacts", exact: true }).click();
  await expect(fixture.getByRole("button", { name: "Operations partner · 跨区域运营协作伙伴", exact: true })).toBeVisible();
  await fixture.getByRole("button", { name: copy(info, "刷新", "Refresh"), exact: true }).click();
  await expect(fixture.getByRole("button", { name: "Toggle stale contacts", exact: true })).toHaveAttribute("aria-pressed", "false");
  expect(errors).toEqual([]);
});

test("Agent identity fields share labels, preserve tag composition and grow long model choices", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-identity-fields]");
  const create = fixture.locator('[data-gallery-identity-variant="dialog"]');
  const inline = fixture.locator('[data-gallery-identity-variant="inline"]');
  const name = create.getByRole("textbox", { name: copy(info, "名称", "Name"), exact: true });
  const inlineName = inline.getByRole("textbox", { name: copy(info, "名称", "Name"), exact: true });
  expect(await name.getAttribute("id")).not.toBe(await inlineName.getAttribute("id"));
  await create.locator("label").filter({ hasText: copy(info, "名称", "Name") }).click();
  await expect(name).toBeFocused();
  for (const label of await fixture.locator("label").all()) {
    const style = await label.evaluate((element) => {
      const css = getComputedStyle(element);
      return { transform: css.textTransform, size: css.fontSize, weight: css.fontWeight };
    });
    expect(style).toEqual({ transform: "none", size: "14px", weight: "500" });
  }
  const business = create.getByRole("textbox", { name: copy(info, "业务标签", "Business tags"), exact: true });
  const tagsShell = business.locator("xpath=../..");
  const heightBefore = (await tagsShell.boundingBox())!.height;
  await business.fill("研究");
  await business.dispatchEvent("keydown", { key: "Enter", isComposing: true });
  await expect(business).toHaveValue("研究");
  await business.press("Enter");
  await expect(business).toHaveValue("");
  await expect(create.getByRole("button", { name: copy(info, "移除 研究", "Remove 研究"), exact: true })).toBeVisible();
  expect((await tagsShell.boundingBox())!.height).toBe(heightBefore);
  expect(heightBefore).toBe(36);
  expect(await tagsShell.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await create.getByRole("button", { name: copy(info, "移除 研究", "Remove 研究"), exact: true }).click();
  await expect(business).toBeFocused();
  await create.getByRole("button", { name: "Toggle name error", exact: true }).click();
  await expect(name).toHaveAttribute("aria-invalid", "true");
  const error = create.getByRole("alert");
  expect(await error.getAttribute("id")).toBe(await name.getAttribute("aria-errormessage"));
  expect(await error.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  const template = create.getByRole("textbox", { name: copy(info, "行为模板", "Behavior template"), exact: true });
  await expect(template).toHaveAccessibleDescription(/AGENTS\.md/);
  await create.getByRole("button", { name: "Toggle template loading" }).click();
  await expect(template).toBeDisabled();
  await expect(inline.getByRole("button", { name: copy(info, "模型", "Model"), exact: true })).toBeDisabled();

  const wrapping = page.locator("[data-gallery-wrapping-selects]");
  for (const size of ["xs", "sm", "md", "lg"]) {
    const trigger = wrapping.getByRole("button", { name: `Wrapping ${size}`, exact: true });
    await trigger.evaluate((element) => { element.parentElement!.style.width = "160px"; });
    const geometry = await trigger.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      const label = element.querySelector("[title]")!;
      const labelBounds = label.getBoundingClientRect();
      return { height: bounds.height, fits: label.scrollWidth <= label.clientWidth + 1 && labelBounds.bottom <= bounds.bottom && labelBounds.top >= bounds.top,
        parentHeight: element.parentElement!.getBoundingClientRect().height };
    });
    expect(geometry.height).toBeGreaterThan(44);
    expect(geometry.fits).toBe(true);
    expect(geometry.parentHeight).toBe(geometry.height);
  }
  await capture(create, info, "agent-identity-fields");
  expect(errors).toEqual([]);
});

test("avatars preserve readable initials and rounded-square member geometry", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const fixture = page.locator("[data-gallery-avatar-geometry]");
  await fixture.scrollIntoViewIfNeeded();
  const failed = fixture.locator("[data-gallery-avatar-failure]");
  await expect(failed).toHaveText("MC");
  await expect(failed.locator("img")).toHaveCount(0);
  const agentMetrics = await fixture.locator("[data-avatar-size]").evaluateAll((avatars) => avatars.map((avatar) => {
    const bounds = avatar.getBoundingClientRect();
    return { width: bounds.width, height: bounds.height, radius: Number.parseFloat(getComputedStyle(avatar).borderTopLeftRadius) };
  }));
  expect(agentMetrics).toEqual([22, 28, 40, 56, 64].map((width, index) => ({ width, height: width, radius: [6, 8, 10, 12, 12][index] })));
  for (const row of await fixture.locator("[data-gallery-room-avatars]").all()) {
    const roomSize = await row.getAttribute("data-gallery-room-avatars");
    for (const count of [1, 2, 4, 9]) {
      const room = row.getByRole("img", { name: `Room ${roomSize} ${count}`, exact: true });
      await expect(room.locator(":scope > span")).toHaveCount(count);
      const geometry = await room.evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        const tiles = Array.from(element.children, (tile) => {
          const rect = tile.getBoundingClientRect();
          const range = document.createRange();
          range.selectNodeContents(tile.firstElementChild!);
          const text = range.getBoundingClientRect();
          return { x: rect.x, y: rect.y, right: rect.right, bottom: rect.bottom, width: rect.width,
            fits: text.width <= rect.width + 1 && text.height <= rect.height + 1 };
        });
        return { left: bounds.left, top: bounds.top, right: bounds.right, bottom: bounds.bottom, tiles };
      });
      for (const tile of geometry.tiles) {
        expect(tile.fits).toBe(true);
        expect(tile.x).toBeGreaterThanOrEqual(geometry.left);
        expect(tile.y).toBeGreaterThanOrEqual(geometry.top);
        expect(tile.right).toBeLessThanOrEqual(geometry.right);
        expect(tile.bottom).toBeLessThanOrEqual(geometry.bottom);
      }
      if (count === 2) {
        expect(geometry.tiles[0].x).toBeLessThan(geometry.tiles[1].x);
        expect(geometry.tiles[0].right).toBeGreaterThan(geometry.tiles[1].x);
      }
    }
  }
  await capture(fixture, info, "avatar-identity-geometry");
  expect(errors).toEqual([]);
});

test("connector details wrap inside narrow panes and preserve named capability dialogs", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-connector-detail]");
  const body = fixture.locator("[data-gallery-connector-detail-body]");
  // Exercise a narrow content pane even when the browser viewport is wide.
  await body.evaluate((element) => { element.style.width = "360px"; });
  const identity = body.locator('[data-slot="capability-detail-identity"]');
  const connect = identity.getByRole("button", { name: copy(info, "添加到 Nexus", "Add to Nexus"), exact: true });
  const title = identity.getByRole("heading");
  await expect(title).toHaveText("RichMail-MultiAccountMailboxAndCalendarConnectorWithAnExtendedName");
  for (const content of [body, title, ...await body.locator("dd").all()]) {
    expect(await content.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  }
  await connect.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, connect);
  const paneBounds = (await body.boundingBox())!;
  const actionBounds = (await connect.boundingBox())!;
  expect(actionBounds.x + actionBounds.width).toBeLessThanOrEqual(paneBounds.x + paneBounds.width + 1);
  await expect(body.getByRole("heading", { name: copy(info, "工具", "Tools"), exact: true })).toHaveCount(0);
  const connectionInfo = body.locator("details").filter({ hasText: copy(info, "连接信息", "Connection details") });
  await expect(connectionInfo).not.toHaveAttribute("open");
  await connectionInfo.locator("summary").click();
  await expect(connectionInfo).toHaveAttribute("open", "");
  for (const value of await connectionInfo.locator("dd").all()) {
    expect(await value.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  }
  const note = body.getByRole("note");
  await expect(note.getByRole("heading")).toHaveCount(1);
  await capture(note, info, "connector-preparation");
  await fixture.locator("[data-gallery-connector-busy]").click();
  await expect(connect).toBeDisabled();
  await fixture.locator("[data-gallery-connector-busy]").click();
  await expect(connect).toBeEnabled();

  await page.setViewportSize({ ...page.viewportSize()!, height: 420 });
  const feature = body.getByRole("button").filter({ hasText: "Mailbox capability" });
  await feature.focus();
  await page.keyboard.press("Enter");
  const dialog = page.getByRole("dialog", { name: "Mailbox capability", exact: true });
  await expectInsideViewport(page, dialog.locator(".dialog-shell"));
  await expect(dialog).toHaveAccessibleDescription(/RichMail/);
  const scopeDisclosure = dialog.locator("details");
  await expect(scopeDisclosure).not.toHaveAttribute("open");
  await scopeDisclosure.locator("summary").click();
  await expect(scopeDisclosure).toHaveAttribute("open", "");
  const scope = dialog.locator("code");
  await expect(scope).toHaveText(`mail:${"account/".repeat(28)}read`);
  expect(await scope.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await capture(dialog.locator(".dialog-shell"), info, "connector-feature-dialog");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(feature).toBeFocused();
  await connect.click();
  await expect(note).toHaveCount(0);
  await expect(body.getByRole("heading", { name: copy(info, "工具", "Tools"), exact: true })).toBeVisible();
  await expect(identity.getByRole("button", { name: copy(info, "断开连接", "Disconnect"), exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

test("Composer WorkGraph picker keeps search focus and reachable selection", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const rejected: string[] = [];
  const graph = { id: "review", title: "Evidence review", slash_name: "review-evidence-with-a-long-reusable-command-name", description: "Verify the full source evidence.",
    objective: "Inspect references", built_in: true, source_execution_id: "", source_session_key: "", nodes: [], dependencies: [], version: 1, created_at: "", updated_at: "" };
  await page.route("**/nexus/v1/**", (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    if (request.method() === "GET" && pathname.endsWith("/workgraph/workflows")) return route.fulfill({ json: { data: [graph, { ...graph, id: "research", title: "Research topic", slash_name: "research" }] } });
    rejected.push(`${request.method()} ${pathname}`);
    return route.abort();
  });
  await page.setViewportSize({ ...page.viewportSize()!, height: 420 });
  await page.evaluate((locale) => localStorage.setItem("nexus-locale", locale), String(info.project.metadata.locale));
  await page.evaluate(async () => {
    const modulePath = "/src/dev/ui-gallery/mount-composer-picker-fixture.ts";
    const { mountComposerPickerFixture } = await import(modulePath);
    mountComposerPickerFixture();
  });
  const dialog = page.getByRole("dialog");
  await expectInsideViewport(page, dialog.locator(".dialog-shell"));
  const search = dialog.getByRole("searchbox");
  await expect(search).toBeFocused();
  expect(await dialog.locator(".dialog-body").evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  const options = dialog.getByRole("option");
  await expect(options).toHaveCount(2);
  await options.first().focus();
  await page.keyboard.press("End");
  await expect(options.last()).toBeFocused();
  await expect(options.last()).toHaveAttribute("aria-selected", "true");
  const use = dialog.getByRole("button", { name: copy(info, "使用这个工作图", "Use this WorkGraph"), exact: true });
  await use.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, use);
  await capture(dialog.locator(".dialog-shell"), info, "composer-workgraph-picker");
  await use.click();
  await expect(page.locator('[data-composer-picker-result="workgraph"]')).toHaveText("/research ");
  await expect(dialog).toHaveCount(0);
  expect(rejected).toEqual([]);
  expect(errors).toEqual([]);
});

test("history menus keep readable metadata, editing focus and reachable batch feedback", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const unexpected: string[] = [];
  await page.route("**/nexus/v1/**", (route) => { unexpected.push(route.request().url()); return route.abort(); });
  await page.setViewportSize({ ...page.viewportSize()!, height: 420 });
  await page.evaluate(async (locale) => {
    localStorage.setItem("nexus-locale", locale);
    const modulePath = "/src/dev/ui-gallery/mount-room-history-fixture.ts";
    const { mountRoomHistoryFixture } = await import(modulePath);
    mountRoomHistoryFixture();
  }, String(info.project.metadata.locale));
  const fixture = page.locator("[data-room-history-fixture]");
  const trigger = fixture.getByRole("button", { name: copy(info, "历史", "History"), exact: true });
  await trigger.click();
  const history = page.getByRole("dialog", { name: copy(info, "历史", "History"), exact: true });
  await expectInsideViewport(page, history);
  const beta = history.getByRole("button", { name: /ResearchEvidenceAndVerification/ });
  await beta.focus();
  const rename = beta.getByRole("button", { name: copy(info, "重命名", "Rename"), exact: true });
  await expect(rename).toHaveCSS("opacity", "1");
  await expect(beta.getByText(copy(info, "刚刚", "Just now"), { exact: true })).toHaveCSS("font-size", "12px");
  await rename.click();
  const input = history.getByRole("textbox");
  await expect(input).toBeFocused();
  await input.fill("Changed title");
  await page.keyboard.press("Escape");
  await expect(input).toHaveCount(0);
  await expect(rename).toBeFocused();
  await expect(history).toBeVisible();
  await expect(fixture.locator("[data-history-commands]")).toHaveText("");
  await history.getByRole("button", { name: copy(info, "多选", "Select"), exact: true }).click();
  await history.getByRole("checkbox", { name: copy(info, "全选", "Select all"), exact: true }).check();
  const clearText = copy(info, "清空历史", "Clear history");
  await history.getByRole("button", { name: clearText, exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: clearText, exact: true }).click();
  const notice = history.getByRole("status");
  await expect(notice).toContainText(copy(info, "不要重复删除", "Do not delete them again"));
  await expect(notice.getByRole("button")).toHaveCount(0);
  const scroll = history.locator("[data-room-history-scroll-viewport]");
  expect(await scroll.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await expectInsideViewport(page, history.getByRole("button", { name: copy(info, "取消", "Cancel"), exact: true }));
  await expect(fixture.locator("[data-history-commands]")).toHaveText("create|delete:beta|delete:alpha");
  await capture(history, info, "history-batch-feedback");
  // Clicking the trigger explicitly closes history without depending on pointer-open tooltips.
  await trigger.click();
  await expect(history).toHaveCount(0);
  await fixture.getByRole("button", { name: "Open mobile history", exact: true }).click();
  const mobile = page.getByRole("dialog", { name: copy(info, "切换会话", "Switch conversation"), exact: true });
  await expectInsideViewport(page, mobile);
  await expect(mobile.getByText(copy(info, "刚刚", "Just now"), { exact: true }).first()).toHaveCSS("font-size", "12px");
  await capture(mobile, info, "mobile-history");
  await mobile.getByRole("button", { name: copy(info, "关闭", "Close"), exact: true }).click();
  await fixture.getByRole("button", { name: "Use empty history", exact: true }).click();
  await fixture.getByRole("button", { name: "Open mobile history", exact: true }).click();
  await expect(mobile.getByRole("status")).toHaveText(copy(info, "暂无对话", "No conversations yet"));
  expect(unexpected).toEqual([]);
  expect(errors).toEqual([]);
});

test("empty form hints remain readable on each theme surface", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const fields = page.locator("[data-gallery-empty-fields]");
  const controls = fields.locator("input, textarea");
  await expect(controls).toHaveCount(4);
  const measurements = [];
  for (const control of await controls.all()) {
    await control.scrollIntoViewIfNeeded();
    const result = await measureTextContrast(control, "::placeholder");
    expect(result.placeholderShown).toBe(true);
    expect(result.ratio, await control.getAttribute("id") ?? "empty field").toBeGreaterThanOrEqual(4.5);
    measurements.push({ id: await control.getAttribute("id"), ...result });
  }
  await info.attach("empty-field-contrast", { body: JSON.stringify(measurements), contentType: "application/json" });
  await capture(fields, info, "empty-fields");
  expect(errors).toEqual([]);
});

test("settings controls share readable compact sizes and preserve keyboard selection and saving locks", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const settings = page.locator("[data-gallery-settings-controls]");
  await settings.scrollIntoViewIfNeeded();
  const modelRow = settings.locator("[data-gallery-settings-model]");
  const permissionRow = settings.locator("[data-gallery-settings-permission]");
  const model = modelRow.getByRole("button", { expanded: false });
  const permission = permissionRow.getByRole("button", { expanded: false });
  const saving = settings.locator("[data-gallery-settings-saving]");
  const commands = settings.locator("[data-gallery-settings-commands]");
  const contrast = [];
  // Flat fixture surfaces isolate the theme's text paint from decorative rain.
  for (const text of await settings.locator("p, .dialog-label, [data-gallery-settings-model] .ui-type-weight-medium").all()) {
    const result = await measureTextContrast(text);
    expect(result.ratio, await text.textContent() ?? "settings text").toBeGreaterThanOrEqual(4.5);
    contrast.push({ text: await text.textContent(), ...result });
  }
  for (const size of ["xs", "sm"]) {
    const group = settings.locator(`[data-gallery-compact-size="${size}"]`);
    const controls = group.locator("input, button");
    await expect(controls).toHaveCount(3);
    for (const control of await controls.all()) {
      const metrics = await control.evaluate((element) => ({
        height: element.getBoundingClientRect().height,
        font: getComputedStyle(element).fontSize,
      }));
      expect(metrics).toEqual({ height: size === "xs" ? 28 : 32, font: size === "xs" ? "12px" : "13px" });
    }
    const field = group.locator("..");
    const description = field.locator(":scope > p");
    await expect(group.getByRole("textbox")).toHaveAccessibleDescription(await description.innerText());
    await expect(group.getByRole("button").first()).not.toHaveAttribute("aria-describedby");
    const gap = (await description.boundingBox())!.y - ((await group.boundingBox())!.y + (await group.boundingBox())!.height);
    expect(gap).toBeCloseTo(8, 1);
    await field.locator("label").click();
    await expect(group.getByRole("textbox")).toBeFocused();
  }
  for (const control of [model, permission]) {
    expect(await control.evaluate((element) => ({ height: element.getBoundingClientRect().height,
      font: getComputedStyle(element).fontSize }))).toEqual({ height: 32, font: "13px" });
  }
  await saving.focus();
  await moveKeyboardFocus(page, info);
  await expect(model).toBeFocused();
  expect(await model.evaluate((element) => element.matches(":focus-visible"))).toBe(true);
  expect(await model.evaluate((element) => getComputedStyle(element).boxShadow)).toMatch(/0px 0px 0px 2px/);
  await capture(model, info, "settings-keyboard-focus");
  await page.keyboard.press("Space");
  await expect(page.getByRole("listbox")).toBeVisible();
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(commands).toHaveText('["agent_runtime:reasoning"]');
  await expect(model).toBeFocused();
  await expect(model).toHaveText(copy(info, "推理模型", "Reasoning model"));
  await moveKeyboardFocus(page, info);
  await expect(permission).toBeFocused();
  await page.keyboard.press("Space");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(commands).toHaveText('["agent_runtime:reasoning","permission:auto"]');
  await saving.click();
  await expect(model).toBeDisabled();
  await expect(permission).toBeDisabled();
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await saving.click();
  await expect(model).toBeEnabled();
  await expect(permission).toBeEnabled();
  await expect(commands).toHaveText('["agent_runtime:reasoning","permission:auto"]');
  expect(await settings.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await info.attach("settings-text-contrast", { body: JSON.stringify(contrast), contentType: "application/json" });
  await capture(settings, info, "settings-compact-controls");
  expect(errors).toEqual([]);
});

test("setting switches retain readable wrapped text, one hit target and independent saving state", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const settings = page.locator("[data-gallery-settings-controls]");
  const row = settings.locator("[data-gallery-settings-toggle]");
  const control = row.getByRole("switch", { name: copy(info, "自动整理记忆", "Automatic memory consolidation") });
  await expect(row.getByRole("heading")).toHaveCSS("font-weight", "500");
  const description = row.locator("p");
  await row.scrollIntoViewIfNeeded();
  await expect(control).toHaveAccessibleDescription(await description.innerText());
  await expect(control).toHaveAttribute("aria-checked", "true");
  expect(await description.evaluate((node) => getComputedStyle(node).fontSize)).toBe("13px");
  const bounds = await control.boundingBox();
  for (const text of [row.getByRole("heading"), description]) {
    const box = (await text.boundingBox())!;
    expect(box.x + box.width).toBeLessThanOrEqual(bounds!.x - 11);
    expect((await measureTextContrast(text)).ratio).toBeGreaterThanOrEqual(4.5);
  }
  expect(await row.evaluate((node) => node.scrollWidth - node.clientWidth)).toBeLessThanOrEqual(1);
  await description.click();
  await expect(settings.locator("[data-gallery-settings-commands]")).toHaveText("[]");
  await control.focus();
  await page.keyboard.press("Space");
  await expect(control).toBeFocused();
  await expect(control).toHaveAttribute("aria-checked", "false");
  await expect(settings.locator("[data-gallery-settings-commands]")).toHaveText('["consolidation:false"]');
  await settings.locator("[data-gallery-settings-saving]").click();
  await expect(control).toBeDisabled();
  await capture(row, info, "settings-toggle-saving");
  await settings.locator("[data-gallery-settings-saving]").click();
  await expect(control).toBeEnabled();
  await expect(control).toHaveAttribute("aria-checked", "false");
  expect(errors).toEqual([]);
});

test("segmented selections share readable density, wrapping, focus hints and disabled states", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-segmented]");
  const lock = fixture.locator("[data-gallery-segmented-lock]");
  for (const groupCase of await fixture.locator('[data-segmented-case$="-text"]').all()) {
    await expect(groupCase.getByRole("group")).toHaveCount(1);
    await expect(groupCase.getByRole("group")).toHaveAccessibleName(/Display mode|显示方式/);
    await expect(groupCase.locator("label button, [role=group] [role=group]")).toHaveCount(0);
  }
  const first = fixture.locator('[data-segmented-case="default-text"]').getByRole("button").first();
  await lock.focus();
  await moveKeyboardFocus(page, info);
  await expect(first).toBeFocused();
  await expect(first).toHaveAttribute("aria-pressed", "true");
  expect(await first.evaluate((element) => ({ visible: element.matches(":focus-visible"), shadow: getComputedStyle(element).boxShadow })))
    .toEqual({ visible: true, shadow: expect.stringMatching(/0px 0px 0px 4px/) });
  await capture(first, info, "segmented-selected-keyboard-focus");
  await moveKeyboardFocus(page, info);
  await page.keyboard.press("Space");
  await expect(fixture.locator("[data-gallery-segmented-commands]")).toHaveText('["default-text:source"]');
  await expect(first).toHaveAttribute("aria-pressed", "false");

  const measurements = [];
  for (const button of await fixture.locator(".segmented-control-option").all()) {
    const groupCase = await button.evaluate((element) => element.closest("[data-segmented-case]")!.getAttribute("data-segmented-case")!);
    const compact = groupCase.startsWith("compact");
    expect(await button.evaluate((element) => getComputedStyle(element).fontSize)).toBe(compact ? "13px" : "14px");
    expect(await button.evaluate((element) => getComputedStyle(element).fontWeight)).toBe("500");
    if (!groupCase.endsWith("long")) expect((await button.boundingBox())!.height).toBe(compact ? 28 : 32);
    const contrast = await measureTextContrast(button);
    expect(contrast.ratio).toBeGreaterThanOrEqual(4.5);
    measurements.push({ ...contrast, ...await button.evaluate((element) => ({
      active: element.getAttribute("aria-pressed"),
      font: getComputedStyle(element).fontSize,
      height: element.getBoundingClientRect().height,
      label: element.textContent,
    })) });
  }
  await info.attach("segmented-control-metrics", { body: JSON.stringify(measurements), contentType: "application/json" });
  for (const button of await fixture.locator('[data-segmented-case$="-mixed"] button').all()) {
    const icon = (await button.locator("svg").boundingBox())!;
    const text = (await button.locator("span").boundingBox())!;
    expect(Math.abs(icon.y + icon.height / 2 - text.y - text.height / 2)).toBeLessThanOrEqual(1);
    expect(text.x).toBeGreaterThan(icon.x + icon.width);
  }
  for (const longCase of await fixture.locator('[data-segmented-case$="-long"]').all()) {
    expect(await longCase.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
    const buttons = longCase.getByRole("button");
    expect((await buttons.first().boundingBox())!.height).toBe((await buttons.last().boundingBox())!.height);
    for (const label of await buttons.locator("span").all()) {
      expect((await label.boundingBox())!.height).toBeGreaterThan(20);
      expect(await label.evaluate((element) => element.scrollHeight <= element.clientHeight + 1 && element.scrollWidth <= element.clientWidth + 1)).toBe(true);
    }
    await longCase.scrollIntoViewIfNeeded();
    await capture(longCase, info, (await longCase.getAttribute("data-segmented-case"))!);
  }
  const icon = fixture.locator('[data-segmented-case="compact-icons"]').getByRole("button").first();
  await icon.focus();
  await expect(page.getByRole("tooltip", { name: copy(info, "预览", "Preview"), exact: true })).toBeVisible();
  await expect(icon).not.toHaveAttribute("title");
  await page.keyboard.press("Escape");
  await expect(page.getByRole("tooltip")).toHaveCount(0);
  await expect(icon).toBeFocused();
  await lock.click();
  for (const button of await fixture.locator(".segmented-control-option").all()) await expect(button).toBeDisabled();
  const beforeHover = await first.evaluate((element) => getComputedStyle(element).backgroundColor);
  await first.hover();
  expect(await first.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(beforeHover);
  for (const groupCase of await fixture.locator("[data-segmented-case]").all()) {
    await groupCase.scrollIntoViewIfNeeded();
    await capture(groupCase, info, `disabled-${await groupCase.getAttribute("data-segmented-case")}`);
  }
  await expect(fixture.locator("[data-gallery-segmented-commands]")).toHaveText('["default-text:source"]');
  expect(await fixture.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  expect(errors).toEqual([]);
});

test("semantic text and actions stay readable on page, card and overlay surfaces", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-semantic-colors]");
  const invalidPaint = await page.evaluate(() => {
    const style = getComputedStyle(document.documentElement);
    return ["--material-chip-background", "--material-input-background", "--material-input-focus-background",
      "--modal-btn-secondary-background", "--button-tonal-background", "--chip-segmented-background"]
      .map((token) => ({ token, value: style.getPropertyValue(token).trim() }))
      .filter(({ value }) => !CSS.supports("background-color", value));
  });
  const measurements = [];
  for (const surface of await fixture.locator("[data-gallery-color-surface]").all()) {
    const name = await surface.getAttribute("data-gallery-color-surface");
    await expect(surface.getByRole("textbox")).toHaveAccessibleErrorMessage(
      await surface.locator('[data-color-sample="field-error"]').innerText(),
    );
    for (const sample of await surface.locator("[data-color-sample]").all()) {
      const id = await sample.getAttribute("data-color-sample");
      const initial = await measureTextContrast(sample);
      measurements.push({ surface: name, id, state: "rest", ...initial });
      // The card also exercises hover for every button variant. Other surfaces
      // cover the same shared recipes at rest with their different backgrounds.
      if (name === "card" && id?.startsWith("button:")) {
        await sample.hover();
        measurements.push({ surface: name, id, state: "hover", ...await measureTextContrast(sample) });
        await page.mouse.move(0, 0);
      }
    }
    expect(await surface.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
    await capture(surface, info, `semantic-colors-${name}`);
  }
  await info.attach("semantic-color-contrast", { body: JSON.stringify(measurements), contentType: "application/json" });
  expect(invalidPaint).toEqual([]);
  expect(measurements.filter(({ ratio }) => ratio < 4.5).map(({ surface, id, state, ratio }) => ({ surface, id, state, ratio }))).toEqual([]);
  expect(errors).toEqual([]);
});

test("WorkGraph inspectors share their surface and preserve exact node and edge actions through zoom", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const graph = page.locator("[data-gallery-workgraph]");
  await graph.scrollIntoViewIfNeeded();
  const draft = graph.locator('[data-execution-graph-node-id="draft"]');
  await draft.click();
  const node = graph.locator('[data-execution-selected-node-detail="draft"]');
  await expect(node).toBeVisible();
  await expect(node.getByRole("heading", { level: 3 })).toHaveText("Draft report");
  const activity = node.locator('[data-execution-runtime-node="evidence"]');
  await expect(activity).toContainText("Read evidence");
  await expect(activity.getByRole("button")).toHaveCount(0);
  expect(await activity.locator(":scope > div").evaluate((element) => getComputedStyle(element).borderRadius)).toBe("10px");
  const metrics = async (inspector: Locator) => inspector.evaluate((element) => {
    const style = getComputedStyle(element);
    const header = getComputedStyle(element.querySelector("header")!);
    return { radius: style.borderRadius, background: style.backgroundColor, headerBackground: header.backgroundColor,
      width: element.getBoundingClientRect().width, font: getComputedStyle(element.querySelector("h3")!).fontSize };
  });
  const initial = await metrics(node);
  expect(initial.radius).toBe("16px");
  expect(initial.font).toBe("12px");
  expect(initial.background).toBe(initial.headerBackground);
  expect(initial.background).not.toBe("rgba(0, 0, 0, 0)");
  await node.getByRole("button", { name: /^review\.md/ }).click();
  await expect(graph.locator("[data-gallery-workgraph-file]")).toHaveText("author:reports/review.md");
  await expect(node).toBeVisible();

  await graph.getByRole("button", { name: copy(info, "放大工作图", "Zoom in"), exact: true }).click();
  const enlarged = await metrics(node);
  expect(enlarged.width).toBeCloseTo(initial.width, 0);
  expect(enlarged.font).toBe(initial.font);
  await capture(node, info, "workgraph-node-inspector");
  await node.getByRole("button", { name: copy(info, "关闭节点详情", "Close node details"), exact: true }).click();
  await expect(node).toHaveCount(0);

  const edgeTrigger = graph.locator('[data-execution-edge-hit-target="draft-review"]');
  await edgeTrigger.focus();
  await page.keyboard.press("Enter");
  const edge = graph.locator('[data-execution-selected-edge-detail="draft-review"]');
  await expect(edge).toContainText("draft-run");
  await expect(edge).toContainText("review-run");
  const edgeMetrics = await metrics(edge);
  expect(edgeMetrics.radius).toBe(initial.radius);
  expect(edgeMetrics.background).toBe(initial.background);
  await capture(edge, info, "workgraph-edge-inspector");
  await page.keyboard.press("Escape");
  await expect(edge).toHaveCount(0);
  await expect(edgeTrigger).toBeFocused();
  const sketch = graph.locator("[data-workgraph-sketch]");
  await expect(sketch.locator("[data-workgraph-sketch-layer]")).toHaveCount(2);
  await expect(sketch.locator('[data-workgraph-sketch-layer="0"] [data-workgraph-sketch-node]')).toHaveAttribute("data-workgraph-sketch-node", "draft");
  await expect(sketch.locator('[data-workgraph-sketch-layer="1"] [data-workgraph-sketch-node]')).toHaveAttribute("data-workgraph-sketch-node", "review");
  await expect(sketch.getByRole("button")).toHaveCount(0);
  await capture(sketch, info, "workgraph-thumbnail");
  expect(errors).toEqual([]);
});

test("private timelines share metadata and message editing preserves keyboard and exact round commands", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-message-surfaces]");
  for (const density of ["compact", "regular"]) {
    const timeline = fixture.locator(`[data-private-timeline-density="${density}"]`);
    await expect(timeline.locator("[data-private-event]")).toHaveCount(3);
    for (const [id, alignment] of [["incoming", "flex-start"], ["outgoing", "flex-end"], ["self", "center"]]) {
      const event = timeline.locator(`[data-private-event="${id}"]`);
      await event.scrollIntoViewIfNeeded();
      const metrics = await event.evaluate((element) => ({
        alignment: getComputedStyle(element).justifyContent,
        radius: getComputedStyle(element.firstElementChild!).borderRadius,
        nameFont: getComputedStyle(element.querySelector(".ui-type-metadata")!).fontSize,
      }));
      expect(metrics).toEqual({ alignment, radius: "12px", nameFont: "12px" });
    }
    await expect.poll(() => timeline.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
    await capture(timeline, info, `private-timeline-${density}`);
  }
  const view = fixture.locator("[data-gallery-message-editor]");
  const commands = fixture.locator("[data-gallery-message-commands]");
  const edit = view.getByRole("button", { name: copy(info, "编辑消息", "Edit message"), exact: true });
  await edit.focus();
  await page.keyboard.press("Enter");
  let input = view.getByRole("textbox");
  await expect(input).toBeFocused();
  await input.fill("Discard this edit");
  await page.keyboard.press("Escape");
  await expect(input).toHaveCount(0);
  await expect(view).toContainText("Original message for editing.");
  await expect(commands).toHaveText("[]");
  await edit.focus();
  await page.keyboard.press("Enter");
  input = view.getByRole("textbox");
  const send = view.getByRole("button", { name: copy(info, "发送", "Send"), exact: true });
  await expect(send).toBeDisabled();
  await input.fill("  Revised line one");
  await page.keyboard.press("Enter");
  await page.keyboard.insertText("line two  ");
  await expect(commands).toHaveText("[]");
  await input.dispatchEvent("keydown", { key: "Enter", ctrlKey: true, isComposing: true, bubbles: true });
  await expect(input).toBeFocused();
  await expect(commands).toHaveText("[]");
  await expect(send).toBeEnabled();
  const height = await input.evaluate((element) => element.getBoundingClientRect().height);
  expect(height).toBeGreaterThanOrEqual(64);
  expect(height).toBeLessThanOrEqual(120);
  await capture(view, info, "user-message-editing");
  await page.keyboard.press("Control+Enter");
  await expect(input).toHaveCount(0);
  await expect(commands).toHaveText(JSON.stringify([{ round: "gallery-round", content: "Revised line one\nline two" }]));
  const reading = fixture.locator("[data-gallery-message-reading]");
  const openedFiles: string[] = [];
  for (const density of ["compact", "expanded"]) {
    const sample = reading.locator(`[data-reading-density="${density}"]`);
    const typography = (surface: Locator) => surface.evaluate((element) => {
      const style = getComputedStyle(element);
      return { font: style.fontSize, lineHeight: style.lineHeight };
    });
    const userBody = await typography(sample.locator(".nexus-chat-user-content"));
    expect(await typography(sample.locator(".nexus-chat-message-content"))).toEqual(userBody);
    expect(userBody).toEqual(density === "compact"
      ? { font: "14px", lineHeight: "24px" }
      : { font: "16px", lineHeight: "28px" });
    const sections = sample.locator(".nexus-chat-message-section");
    const padding = await sections.evaluateAll((elements) => elements.map((element) => getComputedStyle(element).paddingLeft));
    expect(padding[0]).toBe(padding[1]);
    await expect(sample.getByText(copy(info, "已保存到", "Saved to"), { exact: true })).toBeVisible();
    const open = sample.getByRole("button", { name: /^source\.md/ });
    await expect(open).toContainText(copy(info, "打开", "Open"));
    await open.click();
    openedFiles.push("author:reports/source.md");
    await expect(reading.locator("[data-gallery-reading-files]")).toHaveText(JSON.stringify(openedFiles));
    await expect.poll(() => sample.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
    await capture(sample, info, `message-reading-${density}`);
  }
  expect(errors).toEqual([]);
});

test("theme, long labels and button states fit the work plane", async ({ page }, info) => {
  const { gallery, errors } = await openGallery(page, info);
  await expect.poll(() => gallery.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await expect(page.getByRole("button", { name: copy(info, "不可用", "Unavailable"), exact: true })).toBeDisabled();
  const busy = page.getByRole("button", { name: copy(info, "保存中", "Saving"), exact: true });
  await expect(busy).toBeDisabled();
  await expect(busy).toHaveAttribute("aria-busy", "true");

  const primary = page.getByRole("button", { name: copy(info, "新建会话", "New conversation"), exact: true });
  const metrics = await primary.evaluate((element) => {
    const style = getComputedStyle(element);
    return { height: element.getBoundingClientRect().height, font: style.fontSize, weight: style.fontWeight, gap: style.columnGap };
  });
  expect(metrics).toEqual({ height: 36, font: "14px", weight: "500", gap: "8px" });
  const unavailable = page.getByRole("button", { name: copy(info, "不可用", "Unavailable"), exact: true });
  const primaryBackground = await primary.evaluate((element) => getComputedStyle(element).backgroundColor);
  expect(await busy.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(primaryBackground);
  const unavailableBackground = await unavailable.evaluate((element) => getComputedStyle(element).backgroundColor);
  expect(unavailableBackground).not.toBe(primaryBackground);
  await unavailable.hover();
  expect(await unavailable.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(unavailableBackground);
  await primary.scrollIntoViewIfNeeded();
  await primary.focus();
  await moveKeyboardFocus(page, info);
  await moveKeyboardFocus(page, info, true);
  await expect(primary).toBeFocused();
  expect(await primary.evaluate((element) => element.matches(":focus-visible"))).toBe(true);
  expect(await primary.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  const beforeHover = await primary.boundingBox();
  await primary.hover();
  expect(await primary.boundingBox()).toEqual(beforeHover);
  await capture(primary.locator("xpath=ancestor::section[1]"), info, "buttons-focus-hover");
  expect(errors).toEqual([]);
});

test("default form controls share readable typography and aligned field heights", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const input = page.getByRole("textbox", { name: copy(info, "名称", "Name"), exact: true });
  const nativeSelect = page.getByRole("combobox", { name: copy(info, "原生角色", "Native role"), exact: true });
  const select = page.getByRole("button", { name: copy(info, "选择模型", "Choose model"), exact: true });
  const search = page.getByRole("searchbox", { name: copy(info, "搜索", "Search"), exact: true });
  const notes = page.getByRole("textbox", { name: copy(info, "备注", "Notes"), exact: true });
  for (const field of [input, nativeSelect, select, search, notes]) {
    expect(await field.evaluate((element) => getComputedStyle(element).fontSize)).toBe("14px");
    expect(await field.evaluate((element) => getComputedStyle(element).fontWeight)).toBe("400");
  }
  expect(await select.getByText(copy(info, "快速响应模型", "Fast response model"), { exact: true })
    .evaluate((element) => getComputedStyle(element).fontWeight)).toBe("400");
  for (const field of [input, nativeSelect, select, search.locator("..")]) {
    expect((await field.boundingBox())!.height).toBe(36);
  }
  for (const [size, height] of [["sm", 32], ["lg", 44]] as const) {
    const sizedSelect = page.getByRole("button", { name: `Select ${size}`, exact: true });
    expect((await sizedSelect.boundingBox())!.height).toBe(height);
  }
  await input.fill(copy(info, "可读的名称", "Readable name"));
  await expect(input).toHaveValue(copy(info, "可读的名称", "Readable name"));
  await nativeSelect.selectOption("admin");
  await expect(nativeSelect).toHaveValue("admin");
  await search.fill("Nexus");
  await expect(search).toHaveValue("Nexus");
  await search.locator("..").getByRole("button", { name: copy(info, "清除", "Clear"), exact: true }).click();
  await expect(search).toHaveValue("");
  await notes.fill("Nexus\nshared controls");
  await expect(notes).toHaveValue("Nexus\nshared controls");
  // Tall sections extend beyond the Gallery's own scrollport. Capture each
  // visible field instead of attaching an image with clipped, blank lower rows.
  for (const [name, field] of [["input", input], ["search", search], ["select", select], ["notes", notes], ["native-select", nativeSelect]] as const) {
    await field.scrollIntoViewIfNeeded();
    await capture(field.locator("xpath=ancestor::*[contains(@class, 'dialog-field')][1]"), info, `form-${name}`);
  }
  expect(errors).toEqual([]);
});

test("catalog filters share one shape and context usage has one detail surface", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-product-controls]");
  const categories = [
    ["skills", copy(info, "筛选技能分类", "Filter skill categories")],
    ["connectors", copy(info, "筛选连接器分类", "Filter connector categories")],
    ["status", copy(info, "筛选频道状态", "Filter channel status")],
  ].map(([kind, name]) => fixture.locator(`[data-gallery-filter="${kind}"]`).getByRole("button", { name, exact: true }));
  categories.push(page.locator("[data-gallery-contacts]").getByRole("button", {
    name: copy(info, "按业务标签筛选", "Filter by business tag"), exact: true,
  }));
  const metrics = [];
  for (const trigger of categories) {
    await expect(trigger).toHaveCount(1);
    await expect(trigger.locator("svg")).toHaveCount(1);
    metrics.push(await trigger.evaluate((element) => {
      const style = getComputedStyle(element);
      return { height: element.getBoundingClientRect().height, font: style.fontSize,
        radius: style.borderRadius, padding: style.padding, gap: style.gap };
    }));
  }
  expect(metrics[0]).toEqual(metrics[1]);
  expect(metrics[0]).toEqual(metrics[2]);
  expect(metrics[0]).toEqual(metrics[3]);
  await categories[0].click();
  await page.getByRole("option", { name: copy(info, "写作", "Writing"), exact: true }).click();
  await expect(categories[0]).toContainText(copy(info, "写作", "Writing"));
  const selectedLabel = categories[0].getByText(copy(info, "写作", "Writing"), { exact: true });
  expect(await selectedLabel.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await expect(categories[1]).toContainText(copy(info, "全部", "All"));
  await capture(fixture.locator('[data-gallery-filter="skills"]'), info, "catalog-filter-skills");
  await capture(fixture.locator('[data-gallery-filter="connectors"]'), info, "catalog-filter-connectors");

  for (const kind of ["dm", "room"]) {
    const trigger = fixture.locator(`[data-gallery-context="${kind}"]`).getByRole("button");
    await trigger.hover();
    const detail = page.getByRole("tooltip");
    await expect(detail).toHaveCount(1);
    // Wait past UiIconButton's automatic tooltip delay to catch a second surface.
    await page.waitForTimeout(500);
    await expect(detail).toHaveCount(1);
    await expectInsideViewport(page, detail);
    expect(await trigger.getAttribute("aria-describedby")).toBe(await detail.getAttribute("id"));
    if (kind === "room") {
      await expect(detail).toContainText("Reader");
      await expect(detail).toContainText("Writer");
    }
    await capture(detail, info, `context-usage-${kind}`);
    await page.keyboard.press("Escape");
    await expect(detail).toHaveCount(0);
    await trigger.blur();
  }
  expect(errors).toEqual([]);
});

test("Room context details fit three agents and scroll the last row inside a constrained surface", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-product-controls]");

  for (const { kind, count, height } of [
    { kind: "room", count: 3, height: page.viewportSize()!.height },
    { kind: "room-many", count: 12, height: page.viewportSize()!.height },
    { kind: "room-many", count: 12, height: 240 },
  ]) {
    await page.setViewportSize({ width: page.viewportSize()!.width, height });
    const trigger = fixture.locator(`[data-gallery-context="${kind}"]`).getByRole("button");
    await trigger.evaluate((element) => element.scrollIntoView({ block: "end", inline: "nearest" }));
    await trigger.hover();
    const detail = page.getByRole("tooltip");
    const list = detail.getByRole("list", { name: copy(info, "上下文窗口", "Context window") });
    await expect(list.getByRole("listitem")).toHaveCount(count);
    await expectInsideViewport(page, detail);
    expect((await detail.boundingBox())!.width).toBe(232);
    const title = detail.getByText(copy(info, "上下文窗口", "Context window"), { exact: true });
    const titleBounds = await title.boundingBox();

    if (count === 3) {
      // A small list grows to its rendered rows, without either clipping or a tall empty shell.
      expect((await detail.boundingBox())!.height).toBeLessThan(248);
      expect(await list.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeLessThanOrEqual(1);
    } else {
      expect(await list.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(0);
      await list.hover();
      await page.mouse.wheel(0, 3_000);
      await expect.poll(() => list.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    }

    await expect.poll(async () => {
      const surface = (await detail.boundingBox())!;
      const scrollport = (await list.boundingBox())!;
      const lastRow = (await list.getByRole("listitem").last().boundingBox())!;
      return {
        listInsideSurface: scrollport.y >= surface.y && scrollport.y + scrollport.height <= surface.y + surface.height + 1,
        lastRowVisible: lastRow.y >= scrollport.y && lastRow.y + lastRow.height <= scrollport.y + scrollport.height + 1,
      };
    }).toEqual({ listInsideSurface: true, lastRowVisible: true });
    expect(await title.boundingBox()).toEqual(titleBounds);
    await expect(detail).toHaveCount(1);
    await capture(detail, info, `context-height-${kind}-${height}`);
    await page.keyboard.press("Escape");
    await expect(detail).toHaveCount(0);
    await trigger.blur();
  }
  expect(errors).toEqual([]);
});

test("custom MCP fields preserve row identity, technical typography and narrow form geometry", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-custom-mcp]");
  await fixture.getByRole("button").click();
  const dialog = page.getByRole("dialog", { name: copy(info, "编辑自定义 MCP", "Edit custom MCP"), exact: true });
  await expectInsideViewport(page, dialog);
  await expect(dialog.getByRole("group", { name: copy(info, "类型", "Type"), exact: true })).toHaveCount(1);
  const rowName = (index: number) => copy(info, `环境变量第 ${index} 行`, `Environment variables, row ${index}`);
  const keyName = (index: number) => `${rowName(index)} ${copy(info, "键", "Key")}`;
  const valueName = (index: number) => `${rowName(index)} ${copy(info, "值", "Value")}`;
  const removeName = (index: number) => copy(info, `删除${rowName(index)}`, `Delete ${rowName(index)}`);
  const key = dialog.getByLabel(keyName(2), { exact: true });
  const value = dialog.getByLabel(valueName(2), { exact: true });
  await value.scrollIntoViewIfNeeded();
  const keyBounds = (await key.boundingBox())!;
  const valueBounds = (await value.boundingBox())!;
  if (page.viewportSize()!.width < 640) {
    expect(valueBounds.y).toBeGreaterThanOrEqual(keyBounds.y + keyBounds.height);
  } else {
    expect(valueBounds.y).toBe(keyBounds.y);
  }
  for (const input of [key, dialog.getByLabel(copy(info, "启动命令", "Launch command"), { exact: false }),
    dialog.getByRole("textbox", { name: copy(info, "参数第 1 行", "Arguments, row 1"), exact: true })]) {
    expect(await input.evaluate((element) => getComputedStyle(element).fontFamily)).toMatch(/mono/i);
  }
  const id = await value.getAttribute("id");
  await value.fill("replacement");
  await dialog.getByRole("button", { name: removeName(1), exact: true }).click();
  const retained = dialog.getByLabel(valueName(1), { exact: true });
  await expect(retained).toHaveAttribute("id", id!);
  await expect(retained).toHaveValue("replacement");
  await dialog.getByRole("button", { name: copy(info, "添加环境变量", "Add environment variable"), exact: true }).click();
  await dialog.getByLabel(keyName(2), { exact: true }).fill("NEXT");
  await dialog.getByLabel(valueName(2), { exact: true }).fill("new-value");
  expect(await dialog.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await capture(dialog, info, "custom-mcp-dynamic-rows");
  await dialog.getByRole("button", { name: copy(info, "保存", "Save"), exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(fixture).toHaveAttribute("data-save-count", "1");
  expect(errors).toEqual([]);
});

test("Provider configuration follows its container width and distinguishes fixed endpoints", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-provider-form]");
  const host = fixture.locator("[data-gallery-provider-host]");
  const widths = fixture.getByRole("group", { name: copy(info, "表单宽度", "Form width"), exact: true });
  const name = host.getByLabel(copy(info, "服务名称", "Provider Name"), { exact: false });
  const kind = host.getByRole("button", { name: copy(info, "服务类型", "Provider kind"), exact: true });
  const format = host.getByRole("button", { name: copy(info, "接口协议", "API Format"), exact: true });
  for (const width of [320, 560, 800]) {
    const choice = widths.getByRole("button", { name: `${width}px`, exact: true });
    await choice.click();
    await expect(choice).toHaveAttribute("aria-pressed", "true");
    const bounds = (await host.boundingBox())!;
    const [nameBox, kindBox, formatBox] = await Promise.all([name, kind, format].map(async (input) => (await input.boundingBox())!));
    for (const inputBox of [nameBox, kindBox, formatBox]) {
      expect(inputBox.height).toBe(36);
      expect(inputBox.x).toBeGreaterThanOrEqual(bounds.x - 1);
      expect(inputBox.x + inputBox.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1);
    }
    if (bounds.width >= 720) {
      expect(nameBox.y).toBe(kindBox.y);
      expect(kindBox.y).toBe(formatBox.y);
      expect(kindBox.x).toBeGreaterThan(nameBox.x);
      expect(formatBox.x).toBeGreaterThan(kindBox.x);
    } else if (bounds.width >= 480) {
      expect(nameBox.y).toBeLessThan(kindBox.y);
      expect(kindBox.y).toBe(formatBox.y);
      expect(formatBox.x).toBeGreaterThan(kindBox.x);
    } else {
      expect(nameBox.y).toBeLessThan(kindBox.y);
      expect(kindBox.y).toBeLessThan(formatBox.y);
    }
    expect(await host.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
    await host.scrollIntoViewIfNeeded();
    await capture(host, info, `provider-container-${width}`);
  }
  const endpoint = host.getByLabel(copy(info, "服务地址", "Base URL"), { exact: false });
  const beforeBlur = Number(await host.getAttribute("data-blur-count"));
  await endpoint.fill("https://example.com/updated");
  await endpoint.press("Tab");
  await expect(endpoint).toHaveValue("https://example.com/updated");
  await expect(host).toHaveAttribute("data-blur-count", String(beforeBlur + 1));
  await fixture.locator("[data-gallery-provider-fixed]").click();
  const fixed = host.getByRole("group", { name: copy(info, "服务地址", "Base URL"), exact: true });
  await expect(fixed.getByRole("textbox")).toHaveCount(0);
  await expect(fixed.getByRole("button")).toHaveCount(0);
  await expect(fixed).toContainText("https://example.com/api/compatible/chat/completions");
  expect(await fixed.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await capture(fixed, info, "provider-fixed-endpoints");
  expect(errors).toEqual([]);
});

test("Provider model dialogs keep long content scrollable and actions visible in short windows", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  await page.setViewportSize({ width: page.viewportSize()!.width, height: 360 });
  const fixture = page.locator("[data-gallery-provider-dialogs]");
  const dialog = page.getByRole("dialog");
  const body = dialog.locator(".dialog-body--scroll");
  const footer = dialog.locator(".dialog-footer");
  const cancel = dialog.getByRole("button", { name: copy(info, "取消", "Cancel"), exact: true });
  const checkFrame = async () => {
    await expectInsideViewport(page, dialog.locator(".dialog-shell"));
    await expectInsideViewport(page, footer);
    await expect(body).toHaveCount(1);
    expect(await dialog.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
    expect(await body.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  };

  const addTrigger = fixture.locator("[data-gallery-provider-add]");
  await addTrigger.focus();
  await addTrigger.press("Space");
  const model = dialog.getByLabel(copy(info, "模型 ID", "Model ID"), { exact: false });
  await expect(model).toBeFocused();
  expect(await model.evaluate((element) => ({ font: getComputedStyle(element).fontSize,
    mono: /mono/i.test(getComputedStyle(element).fontFamily), height: element.getBoundingClientRect().height,
  }))).toEqual({ font: "14px", mono: true, height: 36 });
  await model.fill("tenant/model");
  await checkFrame();
  await capture(dialog, info, "provider-add-short-window");
  await cancel.click();
  await expect(addTrigger).toBeFocused();

  await fixture.locator("[data-gallery-provider-options]").click();
  const identity = dialog.locator("code");
  await expect(identity).toContainText("extended-context-model-with-a-long-deployment-name");
  expect(await identity.evaluate((element) => ({ overflow: getComputedStyle(element).textOverflow,
    overflowWidth: element.scrollWidth - element.clientWidth }))).toEqual({ overflow: "clip", overflowWidth: 0 });
  const context = dialog.getByLabel(copy(info, "上下文窗口", "Context Window"), { exact: true });
  expect((await context.boundingBox())!.height).toBe(36);
  await context.fill("256000");
  const options = dialog.getByLabel(copy(info, "高级参数（JSON）", "Provider Options (JSON)"), { exact: true });
  await options.fill('{"budget":2}');
  expect(await options.evaluate((element) => getComputedStyle(element).fontFamily)).toMatch(/mono/i);
  const save = dialog.getByRole("button", { name: copy(info, "保存", "Save"), exact: true });
  expect((await save.boundingBox())!.height).toBe(36);
  await checkFrame();
  await save.click();
  const saving = dialog.getByRole("button", { name: copy(info, "保存中...", "Saving..."), exact: true });
  await expect(saving).toBeDisabled();
  await expect(saving).toHaveAttribute("aria-busy", "true");
  await capture(dialog, info, "provider-options-short-window");
  await cancel.click();

  await fixture.locator("[data-gallery-provider-delete]").click();
  await checkFrame();
  const footerBefore = await footer.boundingBox();
  expect(await body.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(0);
  expect(await body.evaluate((element) => [...element.querySelectorAll("*")].filter((child) =>
    /auto|scroll/.test(getComputedStyle(child).overflowY) && child.scrollHeight > child.clientHeight).length)).toBe(0);
  await body.hover();
  await page.mouse.wheel(0, 4_000);
  const lastAgent = body.getByText(/^16 ·/);
  await expect.poll(async () => {
    const area = (await body.boundingBox())!;
    const row = (await lastAgent.boundingBox())!;
    return row.y >= area.y && row.y + row.height <= area.y + area.height + 1;
  }).toBe(true);
  expect(await footer.boundingBox()).toEqual(footerBefore);
  await capture(dialog, info, "provider-usage-last-agent");
  await dialog.getByRole("button", { name: copy(info, "仍要删除", "Delete anyway"), exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(fixture).toHaveAttribute("data-delete-count", "1");
  expect(errors).toEqual([]);
});

test("Skill source and import dialogs keep full metadata, readable guidance and exact local commands", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  await page.setViewportSize({ width: page.viewportSize()!.width, height: 560 });
  const fixture = page.locator("[data-gallery-skill-management]");
  const commands = fixture.locator("[data-gallery-skill-commands]");
  await fixture.getByRole("button", { name: copy(info, "打开来源管理", "Open source manager"), exact: true }).click();
  const manager = page.getByRole("dialog", { name: copy(info, "管理技能来源", "Manage Skill Sources"), exact: true });
  const sourceName = "Engineering research and design systems";
  const sourceUrl = "https://registry.example.test/organization/engineering/design-systems/skills";
  const actions = manager.getByRole("group", { name: sourceName, exact: true });
  const row = actions.locator("..");
  await row.scrollIntoViewIfNeeded();
  for (const [text, size] of [[sourceName, "14px"], [copy(info, "已配置凭据", "Credentials configured"), "13px"],
    [copy(info, "该来源搜索失败。", "This source could not be searched."), "13px"]]) {
    const label = row.getByText(text, { exact: true });
    expect(await label.evaluate((element) => getComputedStyle(element).fontSize)).toBe(size);
    expect(await label.evaluate((element) => element.scrollWidth <= element.clientWidth + 1 && element.scrollHeight <= element.clientHeight + 1)).toBe(true);
  }
  await expect(actions.getByRole("switch")).toHaveAccessibleDescription(new RegExp(sourceUrl.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  expect(await row.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await expectInsideViewport(page, manager.locator(".dialog-footer"));
  await capture(row, info, "skill-source-row");
  await actions.getByRole("button", { name: copy(info, "编辑来源", "Edit source"), exact: true }).click();
  const editor = page.getByRole("dialog", { name: copy(info, "编辑自定义 Skill 来源", "Edit Custom Skill Source"), exact: true });
  const url = editor.getByRole("textbox", { name: copy(info, "服务地址", "Service URL"), exact: true });
  await expect(url).toBeDisabled();
  await expect(url).toHaveValue(sourceUrl);
  const token = editor.getByLabel("Token", { exact: true });
  await expect(token).toHaveValue("");
  await expect(token).toHaveAccessibleDescription(copy(info, "留空会继续使用当前 Token。", "Leave blank to keep the current token."));
  await editor.getByRole("button", { name: copy(info, "验证并保存", "Validate and save"), exact: true }).click();
  await expect.poll(async () => JSON.parse((await commands.textContent())!)).toEqual([{
    type: "save", sourceId: "private-gallery", name: sourceName, url: sourceUrl, authType: "bearer", tokenProvided: false,
  }]);
  await expectInsideViewport(page, editor.locator(".dialog-shell"));
  await expectInsideViewport(page, editor.locator(".dialog-footer"));
  await editor.getByRole("button", { name: copy(info, "取消", "Cancel"), exact: true }).click();
  await manager.locator(".dialog-footer").getByRole("button", { name: copy(info, "关闭", "Close"), exact: true }).click();

  await fixture.getByRole("button", { name: copy(info, "打开 Skill 导入", "Open Skill import"), exact: true }).click();
  const importer = page.getByRole("dialog", { name: copy(info, "导入 Skill", "Import Skill"), exact: true });
  const repository = importer.getByRole("textbox", { name: copy(info, "Git 仓库 URL", "Git repository URL"), exact: true });
  const branch = importer.getByRole("textbox", { name: "Branch", exact: true });
  const path = importer.getByRole("textbox", { name: copy(info, "子目录 Path", "Subdirectory path"), exact: true });
  for (const field of [repository, branch, path]) {
    expect(await field.evaluate((element) => ({ font: getComputedStyle(element).fontSize, mono: /mono/i.test(getComputedStyle(element).fontFamily) })))
      .toEqual({ font: "14px", mono: true });
  }
  await repository.fill("https://example.test/team/repo.git");
  await branch.fill(" feature/skill ");
  await path.fill("  skills/room playbook  ");
  await importer.getByRole("button", { name: copy(info, "本地 zip", "Local zip"), exact: true }).click();
  const fileChooser = page.waitForEvent("filechooser");
  await importer.getByRole("button", { name: copy(info, "选择 zip 文件", "Choose zip file"), exact: true }).click();
  // This fixture records the native file entry only; it does not parse/import an archive.
  await (await fileChooser).setFiles({ name: "fixture.zip", mimeType: "application/zip", buffer: Buffer.from([]) });
  await importer.getByRole("button", { name: copy(info, "Git 仓库", "Git repository"), exact: true }).click();
  await expect(branch).toHaveValue(" feature/skill ");
  await importer.getByRole("button", { name: copy(info, "导入 Git Skill", "Import Git Skill"), exact: true }).click();
  await expect.poll(async () => JSON.parse((await commands.textContent())!).slice(1)).toEqual([
    { type: "file", name: "fixture.zip" }, { type: "import-git", url: "https://example.test/team/repo.git", branch: " feature/skill ", path: "  skills/room playbook  " },
  ]);
  await importer.locator("summary").filter({ hasText: copy(info, "SKILL.md 规范", "SKILL.md requirements") }).click();
  expect(await importer.locator("aside ul").evaluate((element) => getComputedStyle(element).fontSize)).toBe("13px");
  expect(await importer.locator(".dialog-body--scroll").evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await expectInsideViewport(page, importer.locator(".dialog-footer"));
  const rules = await importer.locator("aside li").all();
  for (let index = 0; index < rules.length; index += 1) {
    await rules[index].scrollIntoViewIfNeeded();
    await capture(rules[index], info, `skill-import-rule-${index}`);
  }
  expect(errors).toEqual([]);
});

test("technical fields share monospace presentation and preserve verification zeros", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const path = page.getByRole("textbox", { name: copy(info, "配置路径", "Config path"), exact: true });
  const template = page.getByRole("textbox", { name: copy(info, "源码模板", "Source template"), exact: true });
  const verification = page.getByRole("textbox", { name: copy(info, "验证码", "Verification code"), exact: true });
  for (const field of [path, template, verification]) {
    expect(await field.evaluate((element) => getComputedStyle(element).fontFamily)).toMatch(/mono/i);
  }
  const codeStyle = await verification.evaluate((element) => {
    const style = getComputedStyle(element);
    return { height: element.getBoundingClientRect().height, align: style.textAlign, spacing: Number.parseFloat(style.letterSpacing) };
  });
  expect(codeStyle.height).toBe(48);
  expect(codeStyle.align).toBe("center");
  expect(codeStyle.spacing).toBeGreaterThan(0);
  await verification.fill("002345");
  await expect(verification).toHaveValue("002345");
  await expect(verification).toHaveAttribute("type", "text");
  await path.fill("~/.nexus/workspace");
  await expect(path).toHaveValue("~/.nexus/workspace");
  await template.fill("# Agent\nUse shared controls.");
  await expect(template).toHaveValue("# Agent\nUse shared controls.");
  for (const [name, field] of [["technical-path", path], ["technical-template", template], ["verification", verification]] as const) {
    await field.scrollIntoViewIfNeeded();
    await capture(field.locator("xpath=ancestor::*[contains(@class, 'dialog-field')][1]"), info, name);
  }
  expect(errors).toEqual([]);
});

test("dialog keeps actions visible and returns focus through nested surfaces", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const originalOverflow = await page.locator("body").evaluate((element) => element.style.overflow);
  const trigger = page.getByRole("button", { name: copy(info, "打开标准弹窗", "Open standard dialog"), exact: true });
  // Keyboard activation gives the opener focus on every host. macOS WebKit
  // intentionally does not focus a button on pointer click.
  await trigger.focus();
  await page.keyboard.press("Enter");
  const dialog = page.getByRole("dialog", { name: copy(info, "共享弹窗契约", "Shared dialog contract"), exact: true });
  const titleId = await dialog.getByRole("heading", { level: 2 }).getAttribute("id");
  expect(titleId).toBeTruthy();
  await expect(dialog).toHaveAttribute("aria-labelledby", titleId!);
  const shell = dialog.locator(".dialog-shell");
  await expectInsideViewport(page, shell);
  const close = dialog.getByRole("button", { name: copy(info, "关闭", "Close"), exact: true });
  await expect(close).toBeFocused();
  await expect.poll(() => page.locator("body").evaluate((element) => element.style.overflow)).toBe("hidden");

  const confirm = dialog.getByRole("button", { name: copy(info, "确认", "Confirm"), exact: true });
  await expectInsideViewport(page, confirm);
  await confirm.focus();
  await moveKeyboardFocus(page, info);
  await expect(close).toBeFocused();
  await moveKeyboardFocus(page, info, true);
  await expect(confirm).toBeFocused();
  await capture(shell, info, "dialog");

  const select = dialog.getByRole("button", { name: copy(info, "弹窗内模型", "Model inside dialog"), exact: true });
  await select.click();
  const listbox = page.getByRole("listbox", { name: copy(info, "弹窗内模型", "Model inside dialog"), exact: true });
  await expectInsideViewport(page, listbox);
  // Hit testing proves the portal is above the modal, not merely present in the DOM.
  expect(await listbox.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return element.contains(document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2));
  })).toBe(true);
  await page.keyboard.press("Escape");
  await expect(listbox).toHaveCount(0);
  await expect(dialog).toBeVisible();
  await expect(select).toBeFocused();

  const nestedTrigger = dialog.getByRole("button", { name: copy(info, "打开嵌套确认", "Open nested prompt"), exact: true });
  await nestedTrigger.focus();
  await page.keyboard.press("Enter");
  const nested = page.getByRole("dialog", { name: copy(info, "新建文件夹", "New folder"), exact: true });
  expect(await nested.getAttribute("aria-labelledby")).not.toBe(titleId);
  await expectInsideViewport(page, nested.locator(".dialog-shell"));
  await expect(nested.getByRole("textbox")).toBeFocused();
  await expect(nested.getByRole("textbox")).toHaveAccessibleName(copy(info, "新建文件夹", "New folder"));
  await page.keyboard.press("Escape");
  await expect(nested).toHaveCount(0);
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAttribute("aria-labelledby", titleId!);
  await expect(nestedTrigger).toBeFocused();
  await page.setViewportSize({ ...page.viewportSize()!, height: 420 });
  const multilineTrigger = dialog.getByRole("button", { name: copy(info, "打开多行输入", "Open multiline prompt"), exact: true });
  await multilineTrigger.focus();
  await page.keyboard.press("Enter");
  const multiline = page.getByRole("dialog", { name: copy(info, "补充指令", "Add instruction"), exact: true });
  const textarea = multiline.getByRole("textbox", { name: copy(info, "补充指令", "Add instruction"), exact: true });
  await expect(textarea).toBeFocused();
  await expectInsideViewport(page, multiline.locator(".dialog-shell"));
  await expect(multiline).toHaveAccessibleDescription(copy(info, "输入下一步说明，确认后保留原始换行。", "Describe the next step. Confirmation preserves your line breaks."));
  await expect(textarea).toHaveAccessibleDescription(/Cmd\/Ctrl \+ Enter/);
  await expect(multiline.getByText(/Cmd\/Ctrl \+ Enter/)).toHaveCSS("font-size", "13px");
  await expectInsideViewport(page, multiline.getByRole("button", { name: copy(info, "确认", "Confirm"), exact: true }));
  await textarea.fill("First line");
  await page.keyboard.press("Enter");
  await page.keyboard.type("Second line");
  await expect(textarea).toHaveValue("First line\nSecond line");
  await capture(multiline.locator(".dialog-shell"), info, "multiline-prompt");
  await page.keyboard.press("Control+Enter");
  await expect(multiline).toHaveCount(0);
  await expect(dialog.locator("[data-gallery-prompt-result]")).toHaveText("First line\nSecond line");
  await expect(multilineTrigger).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  expect(await page.locator("body").evaluate((element) => element.style.overflow)).toBe(originalOverflow);
  expect(errors).toEqual([]);
});

test("select handles keyboard, disabled options, viewport edges and outside dismissal", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const trigger = page.getByRole("button", { name: copy(info, "选择模型", "Choose model"), exact: true });
  // Prove the anchor is at the bottom edge before exercising collision handling.
  await trigger.evaluate((element) => {
    element.focus({ preventScroll: true });
    element.closest("main")!.scrollTop += element.getBoundingClientRect().bottom - (window.innerHeight - 16);
  });
  await expect.poll(() => trigger.evaluate((element) => Math.abs(element.getBoundingClientRect().bottom - (window.innerHeight - 16)))).toBeLessThanOrEqual(1);
  await page.keyboard.press("Enter");
  const listbox = page.getByRole("listbox", { name: copy(info, "选择模型", "Choose model"), exact: true });
  await expectInsideViewport(page, listbox);
  await expect(listbox).toHaveAttribute("data-placement", "top");
  // Measure settled geometry; even reduced-motion CSS retains a one-frame entry animation.
  await listbox.evaluate(async (element) => {
    await Promise.all(element.getAnimations().map((animation) => animation.finished));
  });
  await expectInsideViewport(page, listbox);
  const anchorBounds = (await trigger.boundingBox())!;
  const menuBounds = (await listbox.boundingBox())!;
  expect(anchorBounds.y - menuBounds.y - menuBounds.height).toBeGreaterThan(0);
  expect(anchorBounds.y - menuBounds.y - menuBounds.height).toBeLessThanOrEqual(16);
  await page.getByRole("main").evaluate((element) => { element.scrollTop += 40; });
  await expect.poll(async () => {
    const bounds = (await listbox.boundingBox())!;
    return Math.abs(bounds.y - (menuBounds.y - 40));
  }).toBeLessThanOrEqual(1);
  await expect(listbox.getByRole("option", { name: copy(info, "暂不可用的模型", "Temporarily unavailable model"), exact: true })).toBeDisabled();
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(listbox).toHaveCount(0);
  await expect(trigger).toContainText(copy(info, "默认对话模型", "Default conversation model"));
  await expect(trigger).toBeFocused();

  await trigger.click();
  await expectInsideViewport(page, listbox);
  await capture(listbox, info, "select");
  await page.mouse.click(2, 2);
  await expect(listbox).toHaveCount(0);
  expect(errors).toEqual([]);
});

test("action menu focuses, skips disabled rows and returns focus after selection", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "interaction");
  const trigger = page.getByRole("button", { name: copy(info, "打开动作菜单", "Open action menu"), exact: true });
  await trigger.click();
  const menu = page.getByRole("menu", { name: copy(info, "组件动作", "Component actions"), exact: true });
  await expectInsideViewport(page, menu);
  const current = menu.getByRole("menuitem", { name: copy(info, "设为当前", "Set as current"), exact: true });
  const settings = menu.getByRole("menuitem", { name: copy(info, "编辑设置", "Edit settings"), exact: true });
  await expect(current).toBeFocused();
  await expect(menu.getByRole("menuitem", { name: copy(info, "暂不可用", "Temporarily unavailable"), exact: true })).toBeDisabled();
  await page.keyboard.press("ArrowDown");
  await expect(settings).toBeFocused();
  await page.keyboard.press("End");
  await expect(menu.getByRole("menuitem", { name: copy(info, "删除", "Delete"), exact: true })).toBeFocused();
  await page.keyboard.press("Home");
  await expect(current).toBeFocused();
  await capture(menu, info, "action-menu");
  await page.keyboard.press("Enter");
  await expect(menu).toHaveCount(0);
  await expect(trigger).toBeFocused();
  expect(errors).toEqual([]);
});

test("loading stays still in reduced motion and keeps its footprint when animated", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const orb = page.locator("[data-loading-orb=active]").first();
  await orb.scrollIntoViewIfNeeded();
  await expect(orb).toBeVisible();
  const states = await orb.locator(".ui-loading-orb-frame").evaluateAll((frames) => frames.map((frame) => {
    const style = getComputedStyle(frame);
    return { animation: style.animationName, opacity: style.opacity };
  }));
  expect(states.every((state) => state.animation === "none")).toBe(true);
  expect(states.filter((state) => state.opacity === "1")).toHaveLength(1);
  await page.emulateMedia({ reducedMotion: "no-preference" });
  const samples = await orb.evaluate((element) => new Promise<Array<{ glyph: string; width: number; height: number }>>((resolve) => {
    const frames: Array<{ glyph: string; width: number; height: number }> = [];
    const started = performance.now();
    const sample = () => {
      const bounds = element.getBoundingClientRect();
      const glyph = Array.from(element.children).filter((frame) => getComputedStyle(frame).opacity === "1").map((frame) => frame.textContent).join("");
      frames.push({ glyph, width: bounds.width, height: bounds.height });
      if (performance.now() - started >= 700) resolve(frames);
      else requestAnimationFrame(sample);
    };
    requestAnimationFrame(sample);
  }));
  expect(new Set(samples.map((sample) => sample.glyph)).size).toBeGreaterThan(1);
  expect(new Set(samples.map((sample) => `${sample.width}x${sample.height}`)).size).toBe(1);
  expect(errors).toEqual([]);
});

test("scheduled task suggestions fit their pane without overflowing card borders", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const fixture = page.locator("[data-gallery-task-suggestions]");
  for (const width of [260, 520, 760]) {
    await fixture.evaluate((element, value) => { (element as HTMLElement).style.width = `${value}px`; }, width);
    await fixture.scrollIntoViewIfNeeded();
    const cards = fixture.locator(".grid > button");
    await expect(cards).toHaveCount(3);
    const bounds = (await fixture.boundingBox())!;
    const positions = [];
    for (const card of await cards.all()) {
      const rect = (await card.boundingBox())!;
      const description = (await card.locator("span").last().boundingBox())!;
      expect(description.y + description.height).toBeLessThanOrEqual(rect.y + rect.height - 8);
      expect(rect.x + rect.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1);
      expect(await card.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
      positions.push(Math.round(rect.x));
    }
    expect(new Set(positions).size).toBe(bounds.width >= 720 ? 3 : bounds.width >= 480 ? 2 : 1);
  }
  await capture(fixture, info, "task-suggestions");
  expect(errors).toEqual([]);
});

test("scheduled task fields stay named and aligned in narrow panes while preserving draft commands", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const fixture = page.locator("[data-gallery-task-form]");
  const basics = fixture.locator("[data-gallery-task-basics]");
  const schedule = fixture.locator("[data-gallery-task-schedule]");
  const draft = fixture.locator("[data-gallery-task-draft]");
  const location = basics.getByRole("group", { name: copy(info, "执行位置", "Execution location"), exact: true });
  await expect(location).toHaveAccessibleDescription(copy(info,
    "选择由智能体独立处理，或让 Room 成员结合 Room 上下文处理。",
    "Run independently with an agent, or let a Room member work with the Room context."));
  expect(await location.evaluate((element) => getComputedStyle(document.getElementById(element.getAttribute("aria-describedby")!)!).fontSize)).toBe("13px");
  expect(await fixture.evaluate((element) => [...element.querySelectorAll<HTMLLabelElement>("label[for]")]
    .every((label) => label.control && element.contains(label.control)))).toBe(true);

  // Exercise the production panel in a constrained column independently of the Gallery viewport.
  await schedule.evaluate((element) => { (element as HTMLElement).style.width = "min(280px, 100%)"; });
  const kinds = schedule.getByRole("group", { name: copy(info, "调度", "Schedule"), exact: true });
  const bounds = (await schedule.boundingBox())!;
  for (const option of await kinds.getByRole("button").all()) {
    const rect = (await option.boundingBox())!;
    expect(rect.x).toBeGreaterThanOrEqual(bounds.x);
    expect(rect.x + rect.width).toBeLessThanOrEqual(bounds.x + bounds.width + 1);
    expect((await option.locator("span").boundingBox())!.height).toBe(20);
  }
  const amount = schedule.getByRole("spinbutton", { name: copy(info, "每隔", "Every"), exact: true });
  const unit = schedule.getByRole("button", { name: copy(info, "选择间隔单位", "Select interval unit"), exact: true });
  const amountRect = (await amount.boundingBox())!;
  const unitRect = (await unit.boundingBox())!;
  expect(amountRect.height).toBe(36);
  expect(unitRect.height).toBe(36);
  expect(Math.abs(amountRect.y - unitRect.y)).toBeLessThanOrEqual(1);
  await amount.locator("xpath=ancestor::*[@data-ui-field][1]").locator("label").click();
  await expect(amount).toBeFocused();
  await amount.fill("007");
  await unit.click();
  await page.getByRole("option", { name: copy(info, "分钟", "minutes"), exact: true }).click();
  const instruction = schedule.getByRole("textbox", { name: copy(info, "任务指令", "Task instruction"), exact: true });
  await instruction.fill("  Keep\nthese exact lines  ");
  await expect.poll(async () => {
    const state = JSON.parse((await draft.textContent())!);
    return [state.schedule.everyValue, state.schedule.everyUnit, state.form.instruction];
  }).toEqual(["007", "minutes", "  Keep\nthese exact lines  "]);
  expect(await schedule.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await amount.scrollIntoViewIfNeeded();
  await capture(amount.locator("xpath=ancestor::*[@data-ui-field][1]/.."), info, "task-interval-fields");

  await kinds.getByRole("button", { name: copy(info, "每月", "Monthly"), exact: true }).click();
  const day = schedule.getByRole("spinbutton", { name: copy(info, "每月日期", "Day of month"), exact: true });
  await expect(day).toHaveAccessibleDescription(copy(info, "填写 1 至 31；当月没有该日期时，本月不执行。", "Enter 1 to 31. Months without that date are skipped."));
  expect((await day.boundingBox())!.height).toBe(36);
  await kinds.getByRole("button", { name: "Cron", exact: true }).click();
  const cron = schedule.getByRole("textbox", { name: copy(info, "Cron 表达式", "Cron expression"), exact: true });
  await expect(cron).toHaveAccessibleDescription(copy(info,
    "标准五段：分钟 小时 日 月 星期，例如 0 9 15 * *。",
    "Standard five fields: minute hour day month weekday, for example 0 9 15 * *."));
  expect(await cron.evaluate((element) => /mono/i.test(getComputedStyle(element).fontFamily))).toBe(true);
  await cron.fill("  0 9 15 * *  ");
  await expect.poll(async () => JSON.parse((await draft.textContent())!).schedule.cronExpression).toBe("  0 9 15 * *  ");
  await kinds.getByRole("button", { name: copy(info, "每天", "Daily"), exact: true }).click();
  const days = schedule.getByRole("group", { name: copy(info, "执行日", "Run days"), exact: true });
  await expect(days).toHaveAccessibleDescription(copy(info,
    "选中的日期会在这个时间执行；全选就是每天执行。", "Run at this time on the selected days. Select all days to run daily."));
  expect(await schedule.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await days.scrollIntoViewIfNeeded();
  await capture(days, info, "task-weekday-fields");
  expect(errors).toEqual([]);
});

test("controlled workspace tabs preserve selection while creating, pinning and closing", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const tabs = page.getByRole("navigation", { name: copy(info, "对话标签页", "Conversation tabs"), exact: true });
  await tabs.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, tabs);
  const first = tabs.getByRole("button", { name: copy(info, "UI 覆盖", "UI coverage"), exact: true });
  const second = tabs.getByRole("button", { name: copy(info, "响应式文案", "Responsive copy"), exact: true });
  await expect(first).toHaveAttribute("aria-current", "page");
  await second.click();
  await expect(second).toHaveAttribute("aria-current", "page");
  await expect(first).not.toHaveAttribute("aria-current", "page");

  const secondTab = second.locator("xpath=ancestor::*[@data-conversation-tab-id][1]");
  await secondTab.getByRole("button", { name: copy(info, "固定到侧边栏", "Pin to sidebar"), exact: true }).click();
  await expect(secondTab.getByRole("button", { name: copy(info, "从侧边栏取消固定", "Unpin from sidebar"), exact: true })).toBeVisible();
  await expect(second).toHaveAttribute("aria-current", "page");

  await tabs.getByRole("button", { name: copy(info, "新会话", "New session"), exact: true }).click();
  const created = tabs.getByRole("button", { name: copy(info, "新会话 4", "New session 4"), exact: true });
  await expect(created).toHaveAttribute("aria-current", "page");
  await expectInsideViewport(page, created);
  const createdTab = created.locator("xpath=ancestor::*[@data-conversation-tab-id][1]");
  const close = createdTab.getByRole("button", { name: copy(info, "关闭标签页", "Close tab"), exact: true });
  await close.focus();
  await page.keyboard.press("Enter");
  await expect(created).toHaveCount(0);
  const previous = tabs.getByRole("button", { name: copy(info, "键盘状态", "Keyboard states"), exact: true });
  await expect(previous).toHaveAttribute("aria-current", "page");
  await expect(secondTab.getByRole("button", { name: copy(info, "从侧边栏取消固定", "Unpin from sidebar"), exact: true })).toHaveCount(1);
  await capture(tabs, info, "workspace-tabs");
  expect(errors).toEqual([]);
});

test("theme tokens resolve and Tour highlights the real target without swallowing its command", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "interaction");
  const rootTokens = await page.evaluate(() => {
    const root = document.documentElement;
    const names = new Set<string>();
    function inspect(rules: CSSRuleList) {
      for (const rule of Array.from(rules)) {
        if (rule instanceof CSSStyleRule && rule.selectorText.includes(":root") && root.matches(rule.selectorText)) {
          for (const name of Array.from(rule.style)) if (name.startsWith("--")) names.add(name);
        }
        if ("cssRules" in rule) inspect((rule as CSSGroupingRule).cssRules);
      }
    }
    for (const sheet of Array.from(document.styleSheets)) inspect(sheet.cssRules);
    const style = getComputedStyle(root);
    return { count: names.size, empty: [...names].filter((name) => !style.getPropertyValue(name).trim()) };
  });
  expect(rootTokens.count).toBeGreaterThan(150);
  expect(rootTokens.empty).toEqual([]);
  const launch = page.getByRole("button", { name: copy(info, "全屏导览检查", "Preview full tour overlay"), exact: true });
  const target = page.locator('[data-tour-anchor="gallery-tour-target"]');
  await launch.scrollIntoViewIfNeeded();
  await launch.click();
  const highlight = page.locator(".tour-target-highlight");
  await expect(highlight).toBeVisible();
  expect(await highlight.evaluate((element) => getComputedStyle(element).borderRadius)).toBe("10px");
  await expect.poll(async () => {
    const anchor = (await target.boundingBox())!;
    const bounds = (await highlight.boundingBox())!;
    return Math.max(Math.abs(bounds.x - anchor.x + 6), Math.abs(bounds.y - anchor.y + 6), Math.abs(bounds.width - anchor.width - 12), Math.abs(bounds.height - anchor.height - 12));
  }).toBeLessThan(1);
  const card = page.locator("[data-onboarding-tour-card]");
  await expectInsideViewport(page, card);
  await capture(card, info, "tour-target-card");
  await capture(highlight, info, "tour-target-radius");
  await page.keyboard.press("Escape");
  await expect(highlight).toHaveCount(0);
  await launch.click();
  await expect(highlight).toBeVisible();
  await target.click();
  await expect(highlight).toHaveCount(0);
  await expect(page.locator("[data-gallery-tour-actions]")).toHaveText("1");
  expect(errors).toEqual([]);
});

test("Composer draft previews preserve files and keep removal as an independent command", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const fixture = page.locator("[data-gallery-composer-attachments]");
  const shell = fixture.locator("[data-gallery-composer-shell]");
  await shell.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, shell);
  expect(await shell.evaluate((element) => getComputedStyle(element).borderRadius)).toBe("20px");
  const imagePreview = fixture.getByRole("button", { name: /preview-sample.svg/ });
  const textPreview = fixture.getByRole("button", { name: /review-notes-with-a-long-filename.txt/ });
  const thumbnail = imagePreview.locator("..");
  expect((await thumbnail.boundingBox())!.width).toBe(48);
  expect((await thumbnail.boundingBox())!.height).toBe(48);
  const removeImage = fixture.getByRole("button", { name: "Remove draft attachment", exact: true }).first();
  expect((await removeImage.boundingBox())!.width).toBe(20);
  await capture(shell, info, "composer-draft-attachments");

  await imagePreview.focus();
  await page.keyboard.press("Enter");
  const imageDialog = page.getByRole("dialog", { name: "preview-sample.svg", exact: true });
  await expectInsideViewport(page, imageDialog);
  await expect.poll(() => imageDialog.getByRole("img").evaluate((element: HTMLImageElement) => element.naturalWidth)).toBe(480);
  await expect(fixture.locator("[data-gallery-removed-attachments]")).toHaveText("");
  await capture(imageDialog, info, "composer-image-preview");
  await page.keyboard.press("Escape");
  await expect(imagePreview).toBeFocused();

  // Exercise the keyboard focus contract with the host's native traversal.
  // macOS pointer activation does not necessarily focus a button.
  await moveKeyboardFocus(page, info);
  await moveKeyboardFocus(page, info);
  await expect(textPreview).toBeFocused();
  await page.keyboard.press("Enter");
  const textDialog = page.getByRole("dialog", { name: "review-notes-with-a-long-filename.txt", exact: true });
  await expectInsideViewport(page, textDialog);
  await expect(textDialog.locator("pre")).toContainText("<script>window.attachmentExecuted = true</script>");
  expect(await page.evaluate(() => "attachmentExecuted" in window)).toBe(false);
  expect(await textDialog.locator("pre").evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await capture(textDialog, info, "composer-text-preview");
  await page.keyboard.press("Escape");
  await expect(textPreview).toBeFocused();

  await removeImage.click();
  await expect(fixture.locator("[data-gallery-removed-attachments]")).toHaveText("gallery-image");
  await expect(imagePreview).toHaveCount(0);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(textPreview).toBeVisible();
  await fixture.getByRole("button", { name: "Remove draft attachment", exact: true }).first().click();
  await expect(fixture.locator("[data-gallery-removed-attachments]")).toHaveText("gallery-image,gallery-text");
  await expect(textPreview).toHaveCount(0);
  await expect(fixture.getByText("project-archive.zip", { exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});

test("Agent configuration reuses shared rows and cards without widening toggle hit targets", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const controls = page.locator("[data-gallery-agent-options]");
  const permissions = controls.locator("[data-gallery-agent-permissions]");
  const permissionDisclosure = permissions.locator("details");
  await expect(permissionDisclosure).not.toHaveAttribute("open", "");
  await permissionDisclosure.locator("summary").click();
  await expect(permissionDisclosure).toHaveAttribute("open", "");
  for (const card of await permissions.locator("[data-agent-permission-mode]").all()) {
    const description = card.locator("[id$='-description']");
    const title = card.locator("[id$='-title']");
    await expect(description).toHaveCSS("font-size", "13px");
    for (const text of [title, description]) {
      expect(await text.evaluate((element) => element.scrollHeight <= element.clientHeight + 1 && element.scrollWidth <= element.clientWidth + 1)).toBe(true);
    }
    await card.scrollIntoViewIfNeeded();
    await expectInsideViewport(page, card);
  }
  const available = permissions.getByRole("switch", { name: "Available connector", exact: true });
  await expect(available).toHaveAttribute("aria-checked", "false");
  await permissions.getByRole("link", { name: "Available connector", exact: true }).click();
  await expect(available).toHaveAttribute("aria-checked", "false");
  await available.click();
  await expect(available).toHaveAttribute("aria-checked", "true");
  await expect(permissions.getByRole("switch", { name: "Unavailable connector", exact: true })).toBeDisabled();
  const previous = permissions.getByRole("switch", { name: "Previously enabled connector", exact: true });
  await previous.click();
  await expect(previous).toHaveAttribute("aria-checked", "false");
  await expect(previous).toBeDisabled();
  const skills = controls.locator("[data-gallery-agent-skills]");
  const skill = skills.getByRole("switch", { name: "Toggle Review sample", exact: true });
  await skills.getByText("Review sample", { exact: true }).click();
  await expect(skill).toHaveAttribute("aria-checked", "false");
  await skill.click();
  await expect(skill).toHaveAttribute("aria-checked", "true");
  await controls.getByRole("button", { name: "Toggle pending Skill", exact: true }).click();
  await expect(skill).toBeDisabled();
  expect(await skills.getByRole("switch").count()).toBe(1);
  for (const card of await skills.getByRole("article").all()) {
    expect(await card.evaluate((element) => getComputedStyle(element).borderRadius)).toBe("10px");
  }
  await skills.scrollIntoViewIfNeeded();
  await capture(skills, info, "agent-skill-cards");
  const bounds = (await permissions.boundingBox())!;
  expect(bounds.x).toBeGreaterThanOrEqual(0);
  expect(bounds.x + bounds.width).toBeLessThanOrEqual(page.viewportSize()!.width + 1);
  expect(await permissions.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  // This is a scrollable product section, not a viewport-sized modal. Every
  // authorization row must become fully reachable without shrinking the page.
  for (const control of await permissions.getByRole("switch").all()) {
    const row = control.locator("..");
    await row.scrollIntoViewIfNeeded();
    await expectInsideViewport(page, row);
  }
  await available.locator("..").scrollIntoViewIfNeeded();
  await capture(available.locator(".."), info, "agent-authorization-row");
  expect(errors).toEqual([]);
});

test("catalog primary hit area preserves content and independent secondary actions", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "workspace");
  const card = page.getByRole("article", { name: "Catalog action example", exact: true });
  await card.scrollIntoViewIfNeeded();
  await expectInsideViewport(page, card);
  const title = card.getByText("UI Auditor", { exact: true });
  const bounds = (await title.boundingBox())!;
  // Hit the visible content, not an imperatively targeted hidden button.
  await page.mouse.click(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2);
  await expect(page.locator("[data-gallery-catalog-actions]")).toHaveText("1:0");
  await card.getByRole("button", { name: "Catalog secondary action", exact: true }).click();
  await expect(page.locator("[data-gallery-catalog-actions]")).toHaveText("1:1");
  const primary = card.getByRole("button", { name: "Open catalog item", exact: true });
  await primary.focus();
  await page.keyboard.press("Enter");
  await expect(page.locator("[data-gallery-catalog-actions]")).toHaveText("2:1");
  expect(await primary.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  await capture(card, info, "catalog-actions");
  const create = page.getByRole("button", { name: "Create catalog item", exact: true });
  await create.click();
  await expect(page.locator("[data-gallery-catalog-creations]")).toHaveText("1");
  expect(await create.evaluate((element) => getComputedStyle(element).borderRadius)).toBe("12px");
  const unavailable = page.getByRole("button", { name: "Disabled catalog creation", exact: true });
  await unavailable.scrollIntoViewIfNeeded();
  await expect(unavailable).toBeDisabled();
  await page.mouse.move(0, 0);
  const background = await unavailable.evaluate((element) => getComputedStyle(element).backgroundColor);
  await unavailable.hover();
  expect(await unavailable.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(background);
  await expectInsideViewport(page, unavailable);
  await capture(unavailable, info, "catalog-disabled-creation");
  expect(errors).toEqual([]);
});

test("choices share readable sizes, native disabled paint and instance-scoped radio keyboard groups", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const fixture = page.locator("[data-gallery-choices]");
  for (const [size, height, fontSize] of [["xs", 28, 12], ["sm", 32, 13], ["md", 36, 14], ["lg", 40, 14]] as const) {
    const button = fixture.locator(`[data-gallery-choice-size="${size}"]`);
    await expect(button).toHaveCSS("font-size", `${fontSize}px`);
    await expect(button).toHaveCSS("font-weight", "500");
    expect((await button.boundingBox())!.height).toBe(height);
  }
  for (const variant of ["picker", "calendar"]) {
    const button = fixture.getByRole("button", { name: `Choice button ${variant}`, exact: true });
    await button.click();
    await expect(button).toHaveAttribute("aria-pressed", "true");
    await button.evaluate((element) => Promise.all(element.getAnimations().map((animation) => animation.finished)));
    expect((await measureTextContrast(button)).ratio).toBeGreaterThanOrEqual(4.5);
    await expect(button).toHaveCSS("font-size", variant === "picker" ? "16px" : "13px");
    expect((await button.boundingBox())!.height).toBe(variant === "picker" ? 40 : 32);
  }
  await fixture.getByRole("checkbox", { name: copy(info, "允许修改选项", "Enable choice changes"), exact: true }).uncheck();
  for (const variant of ["surface", "picker", "calendar", "icon"]) {
    const button = fixture.getByRole("button", { name: `Choice button ${variant}`, exact: true });
    const radio = fixture.getByRole("radio", { name: `Choice radio ${variant}`, exact: true });
    await expect(button).toBeDisabled();
    await expect(radio).toBeDisabled();
    for (const surface of [button, radio.locator("..")]) {
      await surface.scrollIntoViewIfNeeded();
      await page.mouse.move(0, 0);
      const before = await surface.evaluate((element) => [getComputedStyle(element).backgroundColor, getComputedStyle(element).borderColor]);
      await surface.hover();
      await surface.evaluate((element) => Promise.all(element.getAnimations().map((animation) => animation.finished)));
      await expect(surface).toHaveCSS("cursor", "not-allowed");
      await expect(surface).toHaveCSS("pointer-events", "auto");
      await expect(surface).toHaveCSS("opacity", "0.5");
      expect(await surface.evaluate((element) => [getComputedStyle(element).backgroundColor, getComputedStyle(element).borderColor])).toEqual(before);
    }
  }
  const first = fixture.getByRole("region", { name: "First permission view" });
  const second = fixture.getByRole("region", { name: "Second permission view" });
  const last = first.getByRole("radio", { name: copy(info, "当前工作区", "Current workspace"), exact: true });
  await last.check();
  await last.press("ArrowRight");
  await expect(first.getByRole("radio").first()).toBeFocused();
  await expect(first.getByRole("radio").first()).toBeChecked();
  await expect(second.getByRole("radio", { name: copy(info, "当前会话", "Current session"), exact: true })).toBeChecked();
  expect(await first.getByRole("radio").first().locator("..").evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe("none");
  await capture(fixture.locator("[data-gallery-permission-instances]"), info, "isolated-permission-choices");
  expect(errors).toEqual([]);
});

test("checkbox rows separate names and help, wrap at narrow widths and suppress disabled hover", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const standard = page.getByRole("checkbox", { name: copy(info, "启用完整组件巡检", "Enable complete component audit"), exact: true });
  const fixture = page.locator("[data-gallery-compact-checkbox]");
  await fixture.evaluate((element) => { element.style.width = "min(280px, 100%)"; });
  const compact = fixture.getByRole("checkbox", { name: copy(info, "使用任务独立配置", "Use the task’s independent configuration"), exact: true });
  const row = fixture.locator('[data-slot="checkbox-row"]');
  const help = row.locator("[id$='-description']");
  await expect(compact).toHaveAccessibleDescription(copy(info, "配置只影响当前任务，保存时暂时禁用。", "This configuration only applies to the current task and is disabled while saving."));
  await expect(row.locator("[id$='-label']")).toHaveCSS("font-size", "13px");
  await expect(help).toHaveCSS("font-size", "13px");
  expect(await row.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  await help.click();
  await expect(compact).toBeChecked();
  await expect(compact).toBeFocused();
  await compact.press("Space");
  await expect(compact).not.toBeChecked();

  await standard.uncheck();
  await expect(compact).toBeDisabled();
  await row.scrollIntoViewIfNeeded();
  await page.mouse.move(0, 0);
  const before = await row.evaluate((element) => {
    const style = getComputedStyle(element);
    return [style.backgroundColor, style.borderColor];
  });
  await row.hover();
  await expect(row).toHaveCSS("cursor", "not-allowed");
  await expect.poll(() => row.evaluate((element) => {
    const style = getComputedStyle(element);
    return [style.backgroundColor, style.borderColor];
  })).toEqual(before);
  await help.click();
  await expect(compact).not.toBeChecked();
  await standard.check();
  await help.click();
  await expect(compact).toBeChecked();
  await capture(row, info, "compact-checkbox-row");
  expect(errors).toEqual([]);
});

test("list density and surfaces share geometry and inert rows suppress hover", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const narrow = page.viewportSize()!.width < 560;
  for (const [name, expectedHeight] of [["sidebar", narrow ? 80 : 60], ["sidebar-compact", narrow ? 72 : 54]] as const) {
    const row = page.locator(`[data-gallery-row="${name}"]`);
    await row.scrollIntoViewIfNeeded();
    expect((await row.boundingBox())!.height).toBe(expectedHeight);
    expect(await row.evaluate((element) => getComputedStyle(element).borderRadius)).toBe(narrow ? "12px" : "10px");
    await capture(row, info, `list-${name}`);
  }
  const flush = page.locator('[data-gallery-row="flush"]');
  expect(await flush.evaluate((element) => getComputedStyle(element).borderRadius)).toBe("0px");
  for (const name of ["static", "disabled"]) {
    const row = page.locator(`[data-gallery-row="${name}"]`);
    await page.mouse.move(0, 0);
    const before = await row.evaluate((element) => getComputedStyle(element).backgroundColor);
    await row.hover();
    expect(await row.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(before);
    if (name === "disabled") await expect(row).toHaveAttribute("aria-disabled", "true");
    else await expect(row).not.toHaveAttribute("role");
  }
  expect(errors).toEqual([]);
});

test("list secondary actions reveal for keyboard and suppress disabled hover", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const row = page.locator("[data-gallery-list-actions]");
  await row.scrollIntoViewIfNeeded();
  await page.mouse.move(2, 2);
  const action = row.getByRole("button", { name: "Hover list action", exact: true });
  await expect(action).toHaveCSS("opacity", "0");
  const primary = row.getByRole("button", { name: copy(info, "列表主动作", "List primary action"), exact: true });
  await primary.focus();
  await moveKeyboardFocus(page, info);
  await expect(action).toBeFocused();
  await expect(action).toHaveCSS("opacity", "1");
  const disabled = row.getByRole("button", { name: "Disabled list action", exact: true });
  const background = await disabled.evaluate((element) => getComputedStyle(element).backgroundColor);
  await disabled.hover();
  await expect(disabled).toBeDisabled();
  expect(await disabled.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(background);
  await capture(row, info, "list-actions");
  expect(errors).toEqual([]);
});

test("interactive list rows keep an inset keyboard focus ring through active and disabled neighbors", async ({ page }, info) => {
  const { errors } = await openGallery(page, info);
  const preceding = page.locator("[data-gallery-list-actions]").getByRole("button", { name: "Hover list action", exact: true });
  await preceding.scrollIntoViewIfNeeded();
  await preceding.focus();
  for (const kind of ["sidebar", "sidebar-compact", "flush", "active"]) {
    const row = page.locator(`[data-gallery-row="${kind}"]`);
    const before = await row.boundingBox();
    await moveKeyboardFocus(page, info);
    await expect(row).toBeFocused();
    const after = await row.boundingBox();
    expect(after?.width).toBe(before?.width);
    expect(after?.height).toBe(before?.height);
    expect(await row.evaluate((node) => node.matches(":focus-visible"))).toBe(true);
    expect(await row.evaluate((node) => getComputedStyle(node).boxShadow)).toMatch(/2px.*inset/);
    await capture(row, info, `list-row-${kind}-focus`);
  }
  await expect(page.locator('[data-gallery-row="static"]')).not.toHaveAttribute("tabindex");
  await expect(page.locator('[data-gallery-row="disabled"]')).not.toHaveAttribute("tabindex");
  expect(errors).toEqual([]);
});

test("Contacts directory uses one identity tree, readable metadata and independent actions", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-contacts]");
  const name = "Research and product planning · 跨产品研究规划";
  const card = fixture.getByRole("article").filter({ has: page.getByRole("heading", { name, exact: true }) });
  const title = card.getByRole("heading", { name, exact: true });
  const create = fixture.getByRole("button", { name: copy(info, "新建智能体", "New Agent"), exact: true });
  await expect(fixture.getByRole("searchbox")).toHaveCount(1);
  await expect(create).toHaveCount(1);
  await expect(fixture.getByRole("article")).toHaveCount(3);
  await title.scrollIntoViewIfNeeded();
  expect(await title.evaluate((element) => {
    const style = getComputedStyle(element);
    return { font: style.fontSize, align: style.textAlign, overflow: element.scrollWidth > element.clientWidth + 1,
      clipped: element.scrollHeight > element.clientHeight + 1 };
  })).toEqual({ font: "20px", align: "center", overflow: false, clipped: false });
  expect(await card.locator("dl").evaluate((element) => getComputedStyle(element).fontSize)).toBe("12px");
  const provider = card.getByText("custom-research-model-provider-with-a-long-name", { exact: true });
  expect(await provider.evaluate((element) => element.parentElement!.scrollWidth - element.parentElement!.clientWidth)).toBeLessThanOrEqual(1);
  expect(await card.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  // Static content must route the pointer to the shared primary hit area.
  const titleBounds = (await title.boundingBox())!;
  await page.mouse.click(titleBounds.x + titleBounds.width / 2, titleBounds.y + titleBounds.height / 2);
  const chatLabel = `${copy(info, "聊天", "Chat")} ${name}`;
  const team = card.getByRole("button", { name: `${copy(info, "发起群聊", "Create Team")} ${name}`, exact: true });
  await card.getByRole("button", { name: chatLabel, exact: true }).click();
  await team.focus();
  await page.keyboard.press("Enter");
  const commands = fixture.locator("[data-gallery-contacts-commands]");
  await expect(commands).toHaveText(JSON.stringify(["profile:research", "chat:research", "team:research"]));
  await capture(card, info, "contacts-grid-card");

  const listToggle = fixture.getByRole("button", { name: copy(info, "列表视图", "List view"), exact: true });
  await listToggle.click();
  await expect(fixture.getByRole("article")).toHaveCount(0);
  const row = fixture.getByRole("button", { name: `${copy(info, "编辑", "Edit")} ${name}`, exact: true });
  const listChat = row.getByRole("button", { name: chatLabel, exact: true });
  await listChat.focus();
  await expect(page.getByRole("tooltip")).toHaveText(copy(info, "聊天", "Chat"));
  await expect(listChat).not.toHaveAttribute("title");
  expect(await row.getByRole("heading").evaluate((element) => element.scrollWidth - element.clientWidth)).toBeLessThanOrEqual(1);
  await page.keyboard.press("Escape");
  await capture(row, info, "contacts-list-row");

  await fixture.getByRole("button", { name: copy(info, "按业务标签筛选", "Filter by business tag"), exact: true }).click();
  await page.getByRole("option", { name: "Research and writing", exact: true }).click();
  await expect(fixture.getByRole("heading", { name: "Writer · 写作者", exact: true })).toHaveCount(0);
  await fixture.getByRole("searchbox").fill("no matching agent");
  await expect(fixture.getByRole("status")).toContainText(copy(info, "没有符合当前筛选条件的智能体", "No agents match the current filters"));
  await expect(create).toHaveCount(1);
  await fixture.getByRole("button", { name: copy(info, "清除筛选", "Clear filters"), exact: true }).click();
  await expect(fixture.getByRole("searchbox")).toHaveValue("");
  await expect(listToggle).toHaveAttribute("aria-pressed", "true");
  await expect(fixture.getByRole("heading", { name: "Writer · 写作者", exact: true })).toHaveCount(1);
  expect(errors).toEqual([]);
});

test("Skill details align reading and configuration with concise accessible toggle rows", async ({ page }, info) => {
  const { errors } = await openGallery(page, info, "content");
  const detail = page.locator("[data-gallery-skill-detail]");
  const main = detail.locator('[data-slot="capability-detail-main"]');
  const aside = detail.locator('[data-slot="capability-detail-aside"]');
  const identity = detail.locator('[data-slot="capability-detail-identity"]');
  await expect(identity).not.toContainText("1.1.8");
  const metadata = detail.getByText(/1\.1\.8/);
  const identityBounds = (await identity.boundingBox())!;
  const metadataBounds = (await metadata.boundingBox())!;
  expect(metadataBounds.y).toBeGreaterThanOrEqual(identityBounds.y + identityBounds.height);
  const metadataRow = (await metadata.locator("..").boundingBox())!;
  expect(Math.abs(metadataRow.x - identityBounds.x)).toBeLessThanOrEqual(1);
  await expect(aside).not.toContainText("1.1.8");
  await expect(main.locator(".nexus-workspace-file-markdown")).toHaveCSS("font-size", "14px");
  await expect(aside).not.toContainText(copy(info, "可独立启停", "Can be toggled independently"));
  const switches = aside.getByRole("switch");
  await expect(switches).toHaveCount(3);
  await expect(switches.nth(2)).toBeDisabled();
  await switches.nth(1).focus();
  await page.keyboard.press("Space");
  await expect(switches.nth(1)).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Space");
  await expect(switches.nth(1)).toHaveAttribute("aria-checked", "false");
  expect(await detail.evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  const mainBounds = (await main.boundingBox())!;
  const asideBounds = (await aside.boundingBox())!;
  if (page.viewportSize()!.width >= 1280) {
    expect(Math.abs(mainBounds.y - asideBounds.y)).toBeLessThanOrEqual(1);
    const readingPanel = (await main.locator("section section").boundingBox())!;
    const configurationPanel = (await aside.locator("section section").boundingBox())!;
    expect(Math.abs(readingPanel.y - configurationPanel.y)).toBeLessThanOrEqual(1);
  } else {
    expect(asideBounds.y + asideBounds.height).toBeLessThan(mainBounds.y);
  }
  await capture(detail, info, "skill-detail-layout");
  expect(errors).toEqual([]);
});


test("streaming Markdown catches up across concurrent streams and preserves settled blocks", async ({ page }, info) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  const { errors } = await openGallery(page, info, "content");
  const fixture = page.locator("[data-gallery-streaming-markdown]");
  await fixture.getByRole("button", { name: "Burst", exact: true }).click();
  const outputs = fixture.locator("[data-stream-output]");
  await expect(outputs.first().locator("[data-markdown-anchor]").first()).toHaveText("稳定段落 Markdown 👩🏽‍💻。");
  const stableBlock = await outputs.first().locator("[data-markdown-anchor]").first().elementHandle();
  await fixture.getByRole("button", { name: "Finish", exact: true }).click();
  for (const output of await outputs.all()) {
    await expect(output).toContainText("STREAM_DONE 👩🏽‍💻", { timeout: 2_000 });
    await expect(output.locator("[data-markdown-anchor]")).toHaveCount(12);
    expect(await output.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true);
  }
  expect(await stableBlock?.evaluate((element) => element.isConnected)).toBe(true);
  if (info.project.name === "light-zh-1440") await capture(fixture, info, "streaming-markdown-settled");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await fixture.getByRole("button", { name: "Reset", exact: true }).click();
  await expect(outputs.first()).toHaveText("");
  await fixture.getByRole("button", { name: "Finish", exact: true }).click();
  await expect(outputs.first()).toContainText("STREAM_DONE 👩🏽‍💻");
  expect(errors).toEqual([]);
});

test("glass wordmark animates on hover and settles on leave", async ({ page }, info) => {
  await page.emulateMedia({ reducedMotion: "no-preference" });
  await openGallery(page, info, "interaction");
  const cover = page.getByText("NEXUS", { exact: true }).locator("..");
  await cover.hover();
  const runningLoops = () => cover.evaluate((element) => element.getAnimations({ subtree: true })
    .filter((animation) => animation.playState === "running" && animation.effect?.getTiming().iterations === Infinity).length);
  await expect.poll(runningLoops).toBe(2);
  await page.mouse.move(0, 0);
  await expect.poll(runningLoops).toBe(0);
  await expect.poll(() => cover.evaluate((element) => getComputedStyle(element).transform)).toBe("matrix(1, 0, 0, 1, 0, 0)");
  await page.emulateMedia({ reducedMotion: "reduce" });
  await cover.hover();
  await expect.poll(runningLoops).toBe(0);
});
