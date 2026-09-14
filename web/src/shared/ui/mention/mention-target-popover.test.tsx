// INPUT: Mention 浮层的锚点显示状态、候选项和编辑器键盘事件。
// OUTPUT: 证明隐藏浮层不截获键盘，打开后可选值，关闭后立即释放键盘所有权。
// POS: Mention 键盘生命周期回归测试；不涉及业务目标或文本插入策略。

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, afterEach, beforeAll, expect, it, vi } from "vitest";
import { createRef, useRef, useState, type PropsWithChildren } from "react";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { UiDialogBackdrop, UiDialogPortal, UiDialogShell } from "@/shared/ui/dialog/dialog";

import { MentionTargetPopover } from "./mention-target-popover";

const originalScroll = HTMLElement.prototype.scrollIntoView;
beforeAll(() => { HTMLElement.prototype.scrollIntoView = vi.fn(); });
afterAll(() => { HTMLElement.prototype.scrollIntoView = originalScroll; });
afterEach(() => { vi.restoreAllMocks(); });
function I18n({ children }: PropsWithChildren) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => MESSAGES.zh[key] }}>{children}</I18N_CONTEXT.Provider>;
}

it("captures navigation only while visible and releases it after closing", () => {
  const onSelect = vi.fn();
  const onClose = vi.fn();
  const forwardedKeys: string[] = [];
  const items = [
    { id: "maya", label: "Maya", marker: "M" },
    { id: "lin", label: "Lin", marker: "L" },
  ];
  function Harness({ open }: { open: boolean }) {
    const anchorRef = useRef<HTMLInputElement>(null);
    return (
      <>
        <input ref={anchorRef} aria-label="编辑器" onKeyDown={(event) => forwardedKeys.push(event.key)} />
        <MentionTargetPopover anchorRef={anchorRef} isOpen={open} filter="" items={items} onClose={onClose} onSelect={onSelect} />
      </>
    );
  }
  const view = render(<Harness open={false} />, { wrapper: I18n });
  const editor = screen.getByRole("textbox", { name: "编辑器" });
  const press = (key: string) => {
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key });
    fireEvent(editor, event);
    return event;
  };

  for (const key of ["ArrowDown", "Enter", "Escape"]) {
    expect(press(key).defaultPrevented).toBe(false);
  }
  expect(forwardedKeys).toEqual(["ArrowDown", "Enter", "Escape"]);
  expect(onSelect).not.toHaveBeenCalled();
  expect(onClose).not.toHaveBeenCalled();

  view.rerender(<Harness open />);
  const lin = screen.getByText("Lin").closest("button")!;
  lin.scrollIntoView = vi.fn();
  expect(press("ArrowDown").defaultPrevented).toBe(true);
  expect(press("Enter").defaultPrevented).toBe(true);
  expect(onSelect).toHaveBeenCalledWith(items[1]);
  expect(press("Escape").defaultPrevented).toBe(true);
  expect(onClose).toHaveBeenCalledOnce();
  expect(forwardedKeys).toHaveLength(3);

  view.rerender(<Harness open={false} />);
  expect(press("ArrowDown").defaultPrevented).toBe(false);
  expect(forwardedKeys).toEqual(["ArrowDown", "Enter", "Escape", "ArrowDown"]);
});

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

it("does not consume keys from another editor or IME, and pointer selection fires once without moving focus", async () => {
  const user = userEvent.setup();
  const anchorRef = createRef<HTMLInputElement>();
  const onSelect = vi.fn();
  render(<>
    <input ref={anchorRef} aria-label="输入" />
    <input aria-label="其他" />
    <MentionTargetPopover anchorRef={anchorRef} filter="" items={targets} onClose={vi.fn()} onSelect={onSelect} />
  </>, { wrapper: I18n });
  const editor = screen.getByRole("textbox", { name: "输入" });
  const unrelated = screen.getByRole("textbox", { name: "其他" });
  const otherEvent = new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true });
  fireEvent(unrelated, otherEvent);
  expect(otherEvent.defaultPrevented).toBe(false);
  editor.focus();
  for (const event of [{ key: "Enter", isComposing: true }, { key: "ArrowDown", keyCode: 229 }]) {
    fireEvent.keyDown(editor, event);
  }
  expect(onSelect).not.toHaveBeenCalled();
  expect(screen.getByRole("option", { selected: true }).textContent).toContain("Maya");
  await user.click(screen.getByRole("option", { name: "Lin" }));
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(targets[1]);
  expect(document.activeElement).toBe(editor);
});

it("dismisses on outside pointer input without taking focus from the clicked control", async () => {
  const user = userEvent.setup();
  function Harness() {
    const ref = useRef<HTMLInputElement>(null);
    const [open, setOpen] = useState(true);
    return <>
      <input ref={ref} aria-label="输入" />
      <button>其他动作</button>
      <MentionTargetPopover anchorRef={ref} isOpen={open} filter="" items={targets} onClose={() => setOpen(false)} onSelect={vi.fn()} />
    </>;
  }
  render(<Harness />, { wrapper: I18n });
  screen.getByRole("textbox").focus();
  await user.click(screen.getByRole("button", { name: "其他动作" }));
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "其他动作" }));
  expect(screen.getByRole("textbox").hasAttribute("aria-activedescendant")).toBe(false);
});

it("repositions after scroll, clamps width to the viewport, and closes when filtering removes all choices", () => {
  const anchorRef = createRef<HTMLInputElement>();
  const onClose = vi.fn();
  const view = (filter: string) => <>
    <input ref={anchorRef} aria-label="输入" />
    <MentionTargetPopover anchorRef={anchorRef} filter={filter} items={targets} onClose={onClose} onSelect={vi.fn()} />
  </>;
  const rendered = render(view(""), { wrapper: I18n });
  const anchor = anchorRef.current!;
  const rect = vi.spyOn(anchor, "getBoundingClientRect").mockReturnValue(new DOMRect(window.innerWidth - 70, 100, 500, 32));
  fireEvent.resize(window);
  const list = screen.getByRole("listbox");
  expect(parseFloat(list.style.left) + parseFloat(list.style.width)).toBeLessThanOrEqual(window.innerWidth - 12);
  const firstTop = list.style.top;
  rect.mockReturnValue(new DOMRect(10, window.innerHeight - 40, 500, 32));
  fireEvent.scroll(window);
  expect(list.dataset.placement).toBe("top");
  expect(list.style.top).toBe("auto");
  expect(list.style.top).not.toBe(firstTop);
  expect(parseFloat(list.style.bottom)).toBeGreaterThan(0);
  expect(parseFloat(list.style.left)).toBeGreaterThanOrEqual(12);
  rendered.rerender(view("unmatched"));
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(onClose).toHaveBeenCalledTimes(1);
  expect(anchor.hasAttribute("aria-activedescendant")).toBe(false);
});

it("portals within its dialog and closes only the suggestions before the dialog on Escape", async () => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
  const user = userEvent.setup();
  function Harness() {
    const ref = useRef<HTMLInputElement>(null);
    const [dialog, setDialog] = useState(true);
    const [open, setOpen] = useState(true);
    return dialog ? <UiDialogPortal>
      <UiDialogBackdrop labelledBy="mention-dialog-title" onClose={() => setDialog(false)}>
        <UiDialogShell>
          <h2 id="mention-dialog-title">编辑任务</h2>
          <input ref={ref} aria-label="输入" onKeyDown={(event) => event.stopPropagation()} />
          <MentionTargetPopover anchorRef={ref} isOpen={open} filter="" items={targets} onClose={() => setOpen(false)} onSelect={vi.fn()} />
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal> : null;
  }
  render(<Harness />, { wrapper: I18n });
  const dialog = screen.getByRole("dialog", { name: "编辑任务" });
  const editor = screen.getByRole("textbox");
  editor.focus();
  await waitFor(() => expect(dialog.contains(screen.getByRole("listbox"))).toBe(true));
  fireEvent.keyDown(editor, { key: "Escape", keyCode: 229 });
  expect(screen.getByRole("listbox")).toBeTruthy();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(screen.getByRole("dialog", { name: "编辑任务" })).toBe(dialog);
  expect(document.activeElement).toBe(editor);
  // The editor may stop bubbling; the dialog's own handler still owns its next Escape.
  fireEvent.keyDown(dialog, { key: "Escape" });
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("moves the accessibility association to a replacement editor under the same ref", () => {
  const ref = createRef<HTMLInputElement>();
  const view = (identity: string) => <>
    <input ref={ref} key={identity} aria-label="输入" />
    <MentionTargetPopover anchorRef={ref} filter="" items={targets} onClose={vi.fn()} onSelect={vi.fn()} />
  </>;
  const rendered = render(view("first"), { wrapper: I18n });
  const first = ref.current!;
  const listId = screen.getByRole("listbox").id;
  rendered.rerender(view("second"));
  expect(ref.current).not.toBe(first);
  expect(ref.current?.getAttribute("aria-controls")).toBe(listId);
  expect(ref.current?.getAttribute("aria-activedescendant")).toBe(screen.getByRole("option", { selected: true }).id);
  expect(first.hasAttribute("aria-controls")).toBe(false);
});

it("does not let a background suggestion select while a newer modal is active", () => {
  const ref = createRef<HTMLInputElement>();
  const onSelect = vi.fn();
  render(<>
    <input ref={ref} aria-label="背景" />
    <MentionTargetPopover anchorRef={ref} filter="" items={targets} onClose={vi.fn()} onSelect={onSelect} />
    <UiDialogPortal><UiDialogBackdrop labelledBy="foreground-title" onClose={vi.fn()}><UiDialogShell>
      <h2 id="foreground-title">前台任务</h2><input aria-label="前台输入" />
    </UiDialogShell></UiDialogBackdrop></UiDialogPortal>
  </>, { wrapper: I18n });
  // Even a synthetic event aimed at the old editor must respect the active modal scope.
  fireEvent.keyDown(ref.current!, { key: "Enter" });
  expect(onSelect).not.toHaveBeenCalled();
});
