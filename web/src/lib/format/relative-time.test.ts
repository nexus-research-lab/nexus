// INPUT: Full/compact labels, fixed clock and both product languages.
// OUTPUT: Existing second-precision labels and minute-resolution compact text share unit boundaries.
// POS: Pure elapsed-time formatting regressions.

import { expect, it } from "vitest";
import { formatRelativeTime } from "./relative-time";

const NOW = Date.UTC(2026, 8, 7, 12);
it.each(["en", "zh"] as const)("keeps full precision and localized compact units in %s", (locale) => {
  const intlLocale = locale === "en" ? "en-US" : "zh-CN";
  for (const [seconds, unit] of [[30, "second"], [60, "minute"], [3600, "hour"], [86400, "day"]] as const) {
    const value = seconds === 30 ? -30 : -1;
    expect(formatRelativeTime(NOW - seconds * 1000, locale, { now: NOW })).toBe(new Intl.RelativeTimeFormat(intlLocale, { numeric: "always" }).format(value, unit));
    if (unit === "second") continue;
    expect(formatRelativeTime((NOW - seconds * 1000) / 1000, locale, { compact: true, now: NOW })).toBe(new Intl.RelativeTimeFormat(intlLocale, { numeric: "always", style: "narrow" }).format(value, unit));
  }
});
it("keeps sub-minute and future compact times at just now without invalid arithmetic", () => {
  const justNow = formatRelativeTime(NOW, "en", { compact: true, now: NOW });
  for (const timestamp of [NOW - 59_999, NOW + 60_000, 0, NaN, Infinity]) {
    expect(formatRelativeTime(timestamp, "en", { compact: true, now: NOW })).toBe(justNow);
  }
});
