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
  const bash = permissions.getByRole("switch", { name: "Bash", exact: true });
  await expect(bash).toHaveAttribute("aria-checked", "true");
  await permissions.getByText("Bash", { exact: true }).click();
  await expect(bash).toHaveAttribute("aria-checked", "true");
  await bash.click();
  await expect(bash).toHaveAttribute("aria-checked", "false");
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
  await bash.locator("..").scrollIntoViewIfNeeded();
  await capture(bash.locator(".."), info, "agent-authorization-row");
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
