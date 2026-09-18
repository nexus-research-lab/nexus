// INPUT: 原生 Composer 与已选择 Agent 的草稿。
// OUTPUT: 提及镜像保留输入、换行、滚动与 Slash 渲染。
// POS: Composer 真实浏览器交互回归，不发送消息。
import { expect, test } from "@playwright/test";

test("selected mentions share the native composer mirror with Slash commands", async ({ page }, info) => {
  await page.goto("/ui-gallery.html");
  await page.evaluate(async () => {
    const reactPath = "/node_modules/.vite-browser-test/deps/react.js";
    const domPath = "/node_modules/.vite-browser-test/deps/react-dom_client.js";
    const inputPath = "/src/features/conversation/shared/composer/components/composer-input-row.tsx";
    const mentionPath = "/src/features/conversation/shared/composer/use-composer-mention.ts";
    const { default: React } = await import(reactPath);
    const { default: { createRoot } } = await import(domPath);
    const { ComposerInputRow } = await import(inputPath);
    const { splitComposerMentions } = await import(mentionPath);
    function Harness() {
      const [value, setValue] = React.useState("@Lucy ");
      const textareaRef = React.useRef(null);
      const shellRef = React.useRef(null);
      const noop = () => {};
      return React.createElement("div", { ref: shellRef, style: { position: "fixed", inset: 20, zIndex: 99999, background: "var(--background)" } },
        React.createElement(ComposerInputRow, {
          input: { value, onChange: setValue, placeholder: "Mention draft", disabled: false, onCompositionEnd: noop, onCompositionStart: noop, onKeyDown: noop, onPaste: noop },
          layout: { paddingClassName: "" },
          mention: { active: false, items: [], segments: splitComposerMentions(value, ["Lucy", "Lucy Liu"]), filter: "", onClose: noop, onSelect: noop },
          slashCommand: { active: false }, textareaRef, composerShellRef: shellRef,
        }));
    }
    const host = document.createElement("div");
    document.body.append(host);
    createRoot(host).render(React.createElement(Harness));
  });
  const editor = page.getByRole("textbox", { name: "Mention draft" });
  const mirror = page.locator('[data-composer-text-mirror="true"]');
  await expect(mirror.locator('[data-composer-mention="true"]')).toHaveText("@Lucy");
  const draft = "/goal @Lucy @Lucy Liu，\nhello @Unknown";
  await editor.fill(draft);
  await expect(editor).toHaveValue(draft);
  expect(await mirror.textContent()).toBe(draft);
  await expect(mirror.locator('[data-slash-command-token="true"]')).toHaveText("/goal");
  await expect(mirror.locator('[data-composer-mention="true"]')).toHaveText(["@Lucy", "@Lucy Liu"]);
  await editor.fill("@Luc");
  await expect(mirror).toHaveCount(0);
  await editor.fill("@Lucy\n".repeat(15));
  await editor.evaluate((element) => { element.scrollTop = element.scrollHeight; element.dispatchEvent(new Event("scroll", { bubbles: true })); });
  expect(await mirror.evaluate((element) => element.scrollTop)).toBe(await editor.evaluate((element) => element.scrollTop));
  await editor.fill("@Lucy @Lucy Liu， please review this draft.");
  await page.setViewportSize({ width: 390, height: 600 });
  await expect(editor).toBeFocused();
  await page.screenshot({ path: info.outputPath("composer-mentions.png") });
});
