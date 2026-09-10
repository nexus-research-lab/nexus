// INPUT: Avatar selection, disabled transitions and parent form focus order.
// OUTPUT: Keyboard entry, boundary exit and exact selection regression evidence.
// POS: Shared avatar popover offline interaction tests.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { IconPickerPopover } from "./icon-picker-popover";

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
});
const select = vi.fn();
function Form({ disabled = false, value = "2" }: { disabled?: boolean; value?: string }) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><button>Before</button><IconPickerPopover ariaLabel="Choose avatar" disabled={disabled}
    iconFamily="agent" maxIcons={3} startIconId={1} value={value} onSelect={select}
    renderTrigger={() => "Avatar"} /><input aria-label="Name" /></I18N_CONTEXT.Provider>;
}
it("focuses the selected choice and commits its exact identity", async () => {
  const user = userEvent.setup();
  render(<Form />);
  const trigger = screen.getByRole("button", { name: "Choose avatar" });
  trigger.focus();
  await user.keyboard("{Enter}");
  const choices = within(screen.getByRole("dialog")).getAllByRole("button");
  expect(document.activeElement).toBe(choices[1]);
  await user.keyboard("{Enter}");
  expect(select).toHaveBeenLastCalledWith("2");
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
});
it.each([false, true])("exits in parent form order (backward=%s)", async (backward) => {
  const user = userEvent.setup();
  render(<Form />);
  await user.click(screen.getByRole("button", { name: "Choose avatar" }));
  const choices = within(screen.getByRole("dialog")).getAllByRole("button");
  choices[backward ? 0 : choices.length - 1].focus();
  await user.tab({ shift: backward });
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(backward ? screen.getByRole("button", { name: "Before" }) : screen.getByRole("textbox"));
});
it("closes on disable and value change without reviving old state", async () => {
  const user = userEvent.setup();
  const view = render(<Form />);
  const trigger = screen.getByRole("button", { name: "Choose avatar" });
  await user.click(trigger);
  view.rerender(<Form disabled />);
  expect(screen.queryByRole("dialog")).toBeNull();
  view.rerender(<Form />);
  expect(screen.queryByRole("dialog")).toBeNull();
  await user.click(trigger);
  view.rerender(<Form value="3" />);
  expect(screen.queryByRole("dialog")).toBeNull();
});
it("restores focus on Escape and respects outside click focus", async () => {
  const user = userEvent.setup();
  render(<Form />);
  const trigger = screen.getByRole("button", { name: "Choose avatar" });
  await user.click(trigger);
  await user.keyboard("{Escape}");
  expect(document.activeElement).toBe(trigger);
  await user.click(trigger);
  await user.click(screen.getByRole("textbox"));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("textbox"));
});
