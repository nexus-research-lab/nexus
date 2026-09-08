// INPUT: 窄窗全屏根、真实菜单/子模态与键盘、焦点和关闭事件。
// OUTPUT: 统一层级、焦点循环、逐层 Escape、输入法保护及卸载恢复回归。
// POS: Room 领域外壳的离线行为测试；几何只检查配方所有权，不做视觉验收。

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState } from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiDialogBackdrop, UiDialogPortal } from "@/shared/ui/dialog/dialog";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import { RoomMobileOverlayFrame } from "./room-mobile-overlay-frame";

beforeEach(() => { vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList); });
afterEach(() => vi.restoreAllMocks());

function Harness({ onAction }: { onAction: () => void }) {
  const [open, setOpen] = useState(false);
  const [menu, setMenu] = useState(false);
  const [nested, setNested] = useState(false);
  const anchor = useRef<HTMLButtonElement>(null);
  return <I18nProvider><button onClick={() => setOpen(true)}>Open layer</button>{open ?
    <RoomMobileOverlayFrame label="Focused view" onClose={() => setOpen(false)}>
      <button onClick={() => setOpen(false)}>Back</button>
      <button ref={anchor} aria-expanded={menu} aria-haspopup="menu" onClick={() => setMenu(true)}>Actions</button>
      <button onClick={() => setNested(true)}>Open editor</button>
      <UiActionMenu anchorRef={anchor} ariaLabel="Actions" isOpen={menu} items={[{ value: "act", label: "Run action" }]}
        onClose={() => setMenu(false)} onSelect={onAction} />
      {nested ? <UiDialogPortal><UiDialogBackdrop aria-label="Editor" onClose={() => setNested(false)}>
        <button onClick={() => setNested(false)}>Close editor</button>
      </UiDialogBackdrop></UiDialogPortal> : null}
    </RoomMobileOverlayFrame> : null}</I18nProvider>;
}

it("uses the shared modal root and wraps focus without dismissing on blank content clicks", async () => {
  const user = userEvent.setup();
  const previousOverflow = document.body.style.overflow;
  render(<Harness onAction={vi.fn()} />);
  const opener = screen.getByRole("button", { name: "Open layer" });
  await user.click(opener);
  const dialog = screen.getByRole("dialog", { name: "Focused view" });
  expect(dialog.getAttribute("aria-modal")).toBe("true");
  expect(dialog.getAttribute("data-modal-root")).toBe("true");
  expect(dialog.className).toContain("ui-layer-dialog");
  expect(dialog.className).toContain("min-h-0 min-w-0 flex-col overflow-hidden");
  const back = within(dialog).getByRole("button", { name: "Back" });
  await waitFor(() => expect(document.activeElement).toBe(back));
  expect(document.body.style.overflow).toBe("hidden");
  await user.keyboard("{Shift>}{Tab}{/Shift}");
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "Open editor" }));
  await user.keyboard("{Tab}");
  expect(document.activeElement).toBe(back);
  fireEvent.click(dialog);
  expect(screen.getByRole("dialog")).toBe(dialog);
  fireEvent.keyDown(back, { key: "Escape", isComposing: true });
  expect(screen.getByRole("dialog")).toBe(dialog);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(opener);
  expect(document.body.style.overflow).toBe(previousOverflow);
});

it("keeps menu Portals in the modal and consumes Escape one layer at a time", async () => {
  const user = userEvent.setup();
  const action = vi.fn();
  render(<Harness onAction={action} />);
  await user.click(screen.getByRole("button", { name: "Open layer" }));
  const outer = screen.getByRole("dialog");
  const trigger = within(outer).getByRole("button", { name: "Actions" });
  await user.click(trigger);
  const menu = screen.getByRole("menu");
  expect(outer.contains(menu)).toBe(true);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("menu")).toBeNull();
  expect(screen.getByRole("dialog")).toBe(outer);
  expect(document.activeElement).toBe(trigger);
  expect(action).not.toHaveBeenCalled();
  await user.click(trigger);
  await user.keyboard("{Enter}");
  expect(action).toHaveBeenCalledOnce();
  expect(screen.getByRole("dialog")).toBe(outer);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("keeps the parent locked until the nested editor and then its own layer close", async () => {
  const user = userEvent.setup();
  const original = document.body.style.overflow;
  render(<Harness onAction={vi.fn()} />);
  await user.click(screen.getByRole("button", { name: "Open layer" }));
  const opener = screen.getByRole("button", { name: "Open editor" });
  await user.click(opener);
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button", { name: "Close editor" })));
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("dialog", { name: "Editor" })).toBeNull();
  expect(screen.getByRole("dialog", { name: "Focused view" })).toBeTruthy();
  expect(document.activeElement).toBe(opener);
  expect(document.body.style.overflow).toBe("hidden");
  await user.keyboard("{Escape}");
  expect(document.body.style.overflow).toBe(original);
});

it("focuses an empty root, preserves focus on title updates, and releases the lock on unmount", async () => {
  const close = vi.fn();
  const previousOverflow = document.body.style.overflow;
  const rendered = render(<RoomMobileOverlayFrame label="Loading" onClose={close}><p>Waiting</p></RoomMobileOverlayFrame>);
  const root = screen.getByRole("dialog", { name: "Loading" });
  await waitFor(() => expect(document.activeElement).toBe(root));
  rendered.rerender(<RoomMobileOverlayFrame label="Current title" onClose={close}><p>Ready</p></RoomMobileOverlayFrame>);
  expect(document.activeElement).toBe(root);
  await userEvent.keyboard("{Tab}");
  expect(document.activeElement).toBe(root);
  rendered.unmount();
  expect(document.body.style.overflow).toBe(previousOverflow);
  expect(close).not.toHaveBeenCalled();
});
