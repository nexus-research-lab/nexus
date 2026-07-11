/**
 * API 时间戳转换工具。
 */

export function toTimestampOrNull(value?: string | null): number | null {
  if (!value) {
    return null;
  }

  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : null;
}

export function toTimestamp(value?: string | null): number {
  return toTimestampOrNull(value) ?? 0;
}
