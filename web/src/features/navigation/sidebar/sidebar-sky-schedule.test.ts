import { describe, expect, it } from "vitest";

import { msUntilNextSkyChange, resolveSidebarSkyVariant } from "./sidebar-sky-schedule";

function at(hour: number, minute = 0) {
  return new Date(2026, 10, 26, hour, minute, 0, 0);
}

describe("sidebar sky schedule", () => {
  it.each([
    [0, "meteor"],
    [4, "meteor"],
    [5, "dawn"],
    [8, "dawn"],
    [9, "day"],
    [17, "day"],
    [18, "night"],
    [21, "night"],
    [22, "meteor"],
    [23, "meteor"],
  ])("selects the variant for %i:00", (hour, variant) => {
    expect(resolveSidebarSkyVariant(at(hour, 30))).toBe(variant);
  });

  it("waits until the next slot boundary, crossing midnight", () => {
    expect(msUntilNextSkyChange(at(8, 30))).toBe(30 * 60_000);
    expect(msUntilNextSkyChange(at(23, 0))).toBe(6 * 60 * 60_000);
  });
});
