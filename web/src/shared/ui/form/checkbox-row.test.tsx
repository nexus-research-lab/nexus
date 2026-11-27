// INPUT: Controlled checkbox rows, visible labels/help, caller ARIA and native disabled scopes.
// OUTPUT: Exact names/descriptions, whole-row and keyboard changes, and disabled command isolation.
// POS: Shared checkbox-row regression; browser tests own computed styles and hit geometry.

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UiCheckboxRow } from "./checkbox-row";

describe("UiCheckboxRow", () => {

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
});
