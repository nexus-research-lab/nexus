// INPUT: Controlled segmented choices, native disabled boundaries and keyboard focus.
// OUTPUT: Exact disabled behavior, retained selection and one shared icon-only tooltip.
// POS: Interaction regression; real wrapping, sizes and focus paint belong to browser checks.

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Eye, Code2 } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { UiSegmentedControl } from "./segmented-control";

const options = [{ label: "Preview", value: "preview" }, { label: "Source", value: "source" }];

describe("UiSegmentedControl", () => {
  it.each(["prop", "fieldset"] as const)("retains selection and suppresses native commands while disabled by %s", async (boundary) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const onSubmit = vi.fn((event) => event.preventDefault());
    const view = (disabled: boolean) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><form onSubmit={onSubmit}>
      <fieldset disabled={boundary === "fieldset" && disabled}>
        <UiSegmentedControl disabled={boundary === "prop" && disabled} onChange={onChange}
          options={options} showLabel title="Display" value="preview" />
      </fieldset>
    </form></I18N_CONTEXT.Provider>;
    const { rerender } = render(view(true));
    const group = screen.getByRole("group", { name: "Display" });
    const preview = within(group).getByRole("button", { name: "Preview" });
    const source = within(group).getByRole("button", { name: "Source" });
    expect(preview.matches(":disabled")).toBe(true);
    expect(preview.getAttribute("aria-pressed")).toBe("true");
    await user.click(source);
    expect(onChange).not.toHaveBeenCalled();
    rerender(view(false));
    act(() => source.focus());
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenCalledExactlyOnceWith("source");
    expect(onSubmit).not.toHaveBeenCalled();
    // The owner must supply the next value; focus or a callback cannot invent it.
    expect(preview.getAttribute("aria-pressed")).toBe("true");
  });

  it("gives icon-only choices one shared focus tooltip without native title duplication", () => {
    render(<UiSegmentedControl onChange={vi.fn()} options={[
      { ...options[0], icon: Eye, iconOnly: true }, { ...options[1], icon: Code2, iconOnly: true },
    ]} title="Display" value="preview" />);
    const group = screen.getByRole("group", { name: "Display" });
    const source = within(group).getByRole("button", { name: "Source" });
    expect(group.hasAttribute("title")).toBe(false);
    expect(source.hasAttribute("title")).toBe(false);
    act(() => source.focus());
    const tooltip = screen.getByRole("tooltip", { name: "Source" });
    expect(screen.getAllByRole("tooltip")).toHaveLength(1);
    expect(source.getAttribute("aria-describedby")).toBe(tooltip.id);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(document.activeElement).toBe(source);
  });
});
