// INPUT: Counter values, resource states and recovery actions.
// OUTPUT: Counter limits, live regions, busy state and retry behavior.
// POS: Shared display behavior tests.

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UiCounterBadge } from "@/shared/ui/display/badge";

describe("display badges", () => {
  it("caps positive counters and hides zero", () => {
    const { container, rerender } = render(
      <UiCounterBadge count={120} max={99} />,
    );

    expect(screen.getByText("99+")).toBeTruthy();

    rerender(<UiCounterBadge count={0} />);
    expect(container.textContent).toBe("");
  });
});
