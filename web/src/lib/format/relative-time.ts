// INPUT: Timestamp, current locale and optional compact/minute-resolution clock context.
// OUTPUT: Localized elapsed time through one unit table; existing full labels retain second precision.
// POS: Pure relative-time formatter; rendering lifecycle and refresh cadence belong to consumers.

import type { Locale } from "@/shared/i18n/messages";

interface RelativeTimeUnit {
  seconds: number;
  unit: "day" | "hour" | "minute" | "second";
}

const RELATIVE_TIME_UNITS: RelativeTimeUnit[] = [
  { seconds: 86_400, unit: "day" },
  { seconds: 3_600, unit: "hour" },
  { seconds: 60, unit: "minute" },
  { seconds: 1, unit: "second" },
];

export function formatRelativeTime(
  timestamp: number,
  locale: Locale = "zh",
  { compact = false, now = Date.now() }: { compact?: boolean; now?: number } = {},
): string {
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return formatJustNow(locale);
  }

  const normalizedTimestamp = timestamp < 1_000_000_000_000
    ? timestamp * 1_000
    : timestamp;
  const elapsedSeconds = Math.floor(
    Math.max(0, now - normalizedTimestamp) / 1_000,
  );
  const unit = RELATIVE_TIME_UNITS.find(
    (candidate) => elapsedSeconds >= candidate.seconds
      && (!compact || candidate.unit !== "second"),
  );
  return unit
    ? new Intl.RelativeTimeFormat(resolveIntlLocale(locale), {
      numeric: "always",
      style: compact ? "narrow" : "long",
    }).format(-Math.floor(elapsedSeconds / unit.seconds), unit.unit)
    : formatJustNow(locale);
}

function resolveIntlLocale(locale: Locale): string {
  return locale === "zh" ? "zh-CN" : "en-US";
}

function formatJustNow(locale: Locale): string {
  return locale === "zh" ? "刚刚" : "Just now";
}
