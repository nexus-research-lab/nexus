// INPUT: Select opening gestures and unavailable candidate sets.
// OUTPUT: Resource requests occur synchronously only for accepted opening gestures.
// POS: Public Select onOpen regression; no native permission prompt is invoked.
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiSelectMenu } from "./select-menu";

function setup(options = [{ value: "a", label: "A", disabled: false }], disabled = false) {
  const onOpen = vi.fn();
  const onChange = vi.fn();
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <UiSelectMenu ariaLabel="Font" options={options} value="" onChange={onChange} onOpen={onOpen} disabled={disabled} />
  </I18N_CONTEXT.Provider>);
  return { onOpen, onChange, trigger: screen.getByRole("button", { name: "Font" }) };
}
it.each(["Enter", " ", "ArrowDown", "ArrowUp"])("notifies synchronously once for accepted %s opening", (key) => {
  const { onOpen, trigger } = setup();
  fireEvent.keyDown(trigger, { key });
  expect(onOpen).toHaveBeenCalledOnce();
  expect(screen.getByRole("listbox")).toBeTruthy();
  fireEvent.keyDown(trigger, { key: "Enter" });
  expect(screen.queryByRole("listbox")).toBeNull();
  expect(onOpen).toHaveBeenCalledOnce();
});
it.each([{ options: [] }, { options: [{ value: "a", label: "A", disabled: true }] }])("does not request resources when arrows cannot open", ({ options }) => {
  const { onOpen, onChange, trigger } = setup(options);
  fireEvent.keyDown(trigger, { key: "ArrowDown" });
  fireEvent.keyDown(trigger, { key: "ArrowUp" });
  expect(onOpen).not.toHaveBeenCalled();
  expect(onChange).not.toHaveBeenCalled();
  expect(screen.queryByRole("listbox")).toBeNull();
});
it("ignores disabled gestures", () => {
  const { onOpen, trigger } = setup(undefined, true);
  fireEvent.keyDown(trigger, { key: "Enter" });
  fireEvent.click(trigger);
  expect(onOpen).not.toHaveBeenCalled();
});
it("ignores composition Enter and only reports opening clicks", async () => {
  const { onOpen, trigger } = setup();
  fireEvent.keyDown(trigger, { key: "Enter", isComposing: true });
  expect(onOpen).not.toHaveBeenCalled();
  const user = userEvent.setup();
  await user.click(trigger);
  expect(onOpen).toHaveBeenCalledOnce();
  await user.click(trigger);
  expect(onOpen).toHaveBeenCalledOnce();
});
