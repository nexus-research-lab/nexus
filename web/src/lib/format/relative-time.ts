interface RelativeTimeUnit {
  label: string;
  seconds: number;
}

const RELATIVE_TIME_UNITS: RelativeTimeUnit[] = [
  { label: "天", seconds: 86_400 },
  { label: "小时", seconds: 3_600 },
  { label: "分钟", seconds: 60 },
  { label: "秒", seconds: 1 },
];

export function formatRelativeTime(timestamp: number): string {
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return "刚刚";
  }

  const normalizedTimestamp = timestamp < 1_000_000_000_000
    ? timestamp * 1_000
    : timestamp;
  const elapsedSeconds = Math.floor(
    Math.max(0, Date.now() - normalizedTimestamp) / 1_000,
  );
  const unit = RELATIVE_TIME_UNITS.find(
    (candidate) => elapsedSeconds >= candidate.seconds,
  );
  return unit
    ? `${Math.floor(elapsedSeconds / unit.seconds)}${unit.label}前`
    : "刚刚";
}
