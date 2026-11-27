// INPUT: Controlled segmented choices, native disabled boundaries and keyboard focus.
// OUTPUT: Exact disabled behavior, retained selection and one shared icon-only tooltip.
// POS: Interaction regression; real wrapping, sizes and focus paint belong to browser checks.

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { Eye, Code2 } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { UiSegmentedControl } from "./segmented-control";

const options = [{ label: "Preview", value: "preview" }, { label: "Source", value: "source" }];

describe("UiSegmentedControl", () => {

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
