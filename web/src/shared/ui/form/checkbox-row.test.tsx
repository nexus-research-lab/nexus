// INPUT: Controlled checkbox rows, visible labels/help, caller ARIA and native disabled scopes.
// OUTPUT: Exact names/descriptions, whole-row and keyboard changes, and disabled command isolation.
// POS: Shared checkbox-row regression; browser tests own computed styles and hit geometry.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiCheckboxRow } from "./checkbox-row";

describe("UiCheckboxRow", () => {
  it.each(["default", "compact"] as const)("names the %s row separately from its clickable description", async (density) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    function Harness() {
      const [checked, setChecked] = useState(false);
      return <>
        <p id="outside-help">Additional context</p>
        <UiCheckboxRow
          aria-describedby="outside-help"
          checked={checked}
          density={density}
          description="Keep this task enabled."
          icon={<svg><title>Decorative icon</title></svg>}
          label="Enable task"
          onChange={(next) => { setChecked(next); onChange(next); }}
        />
      </>;
    }
    render(<Harness />);
    const input = screen.getByRole("checkbox", { name: "Enable task" }) as HTMLInputElement;
    const descriptions = input.getAttribute("aria-describedby")!.split(" ");
    expect(descriptions.map((id) => document.getElementById(id)?.textContent))
      .toEqual(["Additional context", "Keep this task enabled."]);
    await user.click(screen.getByText("Keep this task enabled."));
    expect(input.checked).toBe(true);
    expect(onChange).toHaveBeenCalledExactlyOnceWith(true);
    expect(document.activeElement).toBe(input);
    await user.keyboard(" ");
    expect(input.checked).toBe(false);
    expect(onChange.mock.calls).toEqual([[true], [false]]);
  });

  it("preserves explicit names and isolates descriptions across instances and updates", () => {
    function Harness({ description }: { description?: string }) {
      return <>
        <p id="explicit-name">External name</p>
        <UiCheckboxRow aria-label="Caller name" checked={false} description={description} label="Visible one" onChange={vi.fn()} />
        <UiCheckboxRow aria-labelledby="explicit-name" checked={false} description="Second help" label="Visible two" onChange={vi.fn()} />
      </>;
    }
    const { rerender } = render(<Harness description="First help" />);
    const first = screen.getByRole("checkbox", { name: "Caller name" });
    const second = screen.getByRole("checkbox", { name: "External name" });
    expect(first.getAttribute("aria-describedby")).not.toBe(second.getAttribute("aria-describedby"));
    expect(document.getElementById(first.getAttribute("aria-describedby")!)?.textContent).toBe("First help");
    rerender(<Harness />);
    expect(first.hasAttribute("aria-describedby")).toBe(false);
    expect(document.getElementById(second.getAttribute("aria-describedby")!)?.textContent).toBe("Second help");
  });

  it.each(["input", "fieldset"])("keeps a row disabled by its %s inert and outside keyboard order", async (scope) => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    function Harness({ disabled }: { disabled: boolean }) {
      return <>
        <fieldset disabled={scope === "fieldset" && disabled}>
          <UiCheckboxRow checked disabled={scope === "input" && disabled} description="Task explanation" label="Enable task" onChange={onChange} />
        </fieldset>
        <button>Next action</button>
      </>;
    }
    const { rerender } = render(<Harness disabled />);
    const input = screen.getByRole("checkbox", { name: "Enable task" }) as HTMLInputElement;
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Next action" }));
    await user.click(screen.getByText("Enable task"));
    await user.click(screen.getByText("Task explanation"));
    expect(input.checked).toBe(true);
    expect(onChange).not.toHaveBeenCalled();
    rerender(<Harness disabled={false} />);
    await user.click(screen.getByText("Task explanation"));
    expect(onChange).toHaveBeenCalledExactlyOnceWith(false);
  });
});
