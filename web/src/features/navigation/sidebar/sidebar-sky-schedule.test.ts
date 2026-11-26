import { describe, expect, it } from "vitest";

import { msUntilNextSkyChange, resolveSidebarSkyVariant } from "./sidebar-sky-schedule";

function at(hour: number, minute = 0) {
  return new Date(2026, 10, 26, hour, minute, 0, 0);
}

describe("sidebar sky schedule", () => {
  it.each([
    [0, "meteor"],
    [1, "night"],
    [4, "night"],
    [5, "dawn"],
    [8, "dawn"],
    [9, "constellation"],
    [17, "constellation"],
    [18, "aurora"],
    [21, "aurora"],
    [22, "meteor"],
    [23, "meteor"],
  ])("selects the variant for %i:00", (hour, variant) => {
    expect(resolveSidebarSkyVariant(at(hour, 30))).toBe(variant);
  });

  it("waits until the next slot boundary, crossing midnight", () => {
    expect(msUntilNextSkyChange(at(8, 30))).toBe(30 * 60_000);
    expect(msUntilNextSkyChange(at(23, 0))).toBe(2 * 60 * 60_000);
  });
});
