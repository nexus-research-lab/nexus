// INPUT: Mention 浮层的锚点显示状态、候选项和编辑器键盘事件。
// OUTPUT: 证明隐藏浮层不截获键盘，打开后可选值，关闭后立即释放键盘所有权。
// POS: Mention 键盘生命周期回归测试；不涉及业务目标或文本插入策略。

import { fireEvent, render, screen } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { createRef, type PropsWithChildren } from "react";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";

import { MentionTargetPopover } from "./mention-target-popover";

const originalScroll = HTMLElement.prototype.scrollIntoView;
beforeAll(() => { HTMLElement.prototype.scrollIntoView = vi.fn(); });
afterAll(() => { HTMLElement.prototype.scrollIntoView = originalScroll; });
afterEach(() => { vi.restoreAllMocks(); });
function I18n({ children }: PropsWithChildren) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => MESSAGES.zh[key] }}>{children}</I18N_CONTEXT.Provider>;
}

const targets = [
  { id: "maya", label: "Maya", marker: "M", subtitle: "Frontend" },
  { id: "lin", label: "Lin", marker: "L" },
];

it("links active options to the focused editor and restores previous attributes on close", () => {
  const anchorRef = createRef<HTMLInputElement>();
  const view = (open: boolean) => <>
    <input ref={anchorRef} aria-label="输入" aria-autocomplete="both" aria-controls="existing" />
    <MentionTargetPopover anchorRef={anchorRef} isOpen={open} filter="" items={targets} onClose={vi.fn()} onSelect={vi.fn()} />
  </>;
  const rendered = render(view(true), { wrapper: I18n });
  const editor = screen.getByRole("textbox", { name: "输入" });
  const list = screen.getByRole("listbox", { name: "提及候选" });
  editor.focus();
  expect(editor.getAttribute("aria-controls")).toBe(list.id);
  expect(editor.getAttribute("aria-activedescendant")).toBe(screen.getByRole("option", { selected: true }).id);
  fireEvent.keyDown(editor, { key: "ArrowDown" });
  expect(screen.getByRole("option", { selected: true }).textContent).toContain("Lin");
  expect(editor.getAttribute("aria-activedescendant")).toBe(screen.getByRole("option", { selected: true }).id);
  expect(document.activeElement).toBe(editor);
  rendered.rerender(view(false));
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(editor.getAttribute("aria-controls")).toBe("existing");
  expect(editor.getAttribute("aria-autocomplete")).toBe("both");
  expect(editor.hasAttribute("aria-activedescendant")).toBe(false);
});
