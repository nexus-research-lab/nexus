// INPUT: 真实 shared UI Gallery、浏览器布局/焦点与主题语言视口矩阵。
// OUTPUT: 页面不溢出、模态焦点/滚动隔离、最上层 Escape、菜单定位及减弱动效的回归证据。
// POS: 浏览器行为门禁；截图用于人工复核，不把截图生成或 jsdom 当作像素比对结论。

import { expect, test, type Locator, type Page, type TestInfo } from "@playwright/test";

import { moveKeyboardFocus } from "./keyboard";

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
  await expectInsideViewport(page, nested.locator(".dialog-shell"));
  await expect(nested.getByRole("textbox")).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(nested).toHaveCount(0);
  await expect(dialog).toBeVisible();
  await expect(nestedTrigger).toBeFocused();
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
