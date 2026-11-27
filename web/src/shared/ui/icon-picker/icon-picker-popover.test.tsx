// INPUT: Avatar selection, disabled transitions and parent form focus order.
// OUTPUT: Keyboard entry, boundary exit and exact selection regression evidence.
// POS: Shared avatar popover offline interaction tests.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { getIconAvatarSrc } from "@/lib/avatar";
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
it("previews repeated random choices and commits only the last on outside click", async () => {
  select.mockClear();
  const user = userEvent.setup();
  render(<Form />);
  await user.click(screen.getByRole("button", { name: "Choose avatar" }));
  const dialog = screen.getByRole("dialog");
  expect(screen.queryByRole("combobox")).toBeNull();
  const random = screen.getByRole("button", { name: "common.avatar_randomize" });
  await user.click(random);
  const first = dialog.querySelector("img")?.getAttribute("src");
  await user.click(random);
  const last = dialog.querySelector("img")?.getAttribute("src");
  expect(last).not.toBe(first);
  expect(screen.getByRole("dialog")).toBe(dialog);
  expect(document.activeElement).toBe(random);
  expect(select).not.toHaveBeenCalled();
  await user.click(screen.getByRole("textbox"));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(select).toHaveBeenCalledTimes(1);
  expect(getIconAvatarSrc(select.mock.calls[0][0])).toBe(last);
  expect(document.activeElement).toBe(screen.getByRole("textbox"));
});
