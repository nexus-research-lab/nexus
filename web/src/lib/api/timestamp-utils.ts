/**
 * API 时间戳转换工具。
 */

export function to_timestamp_or_null(value?: string | null): number | null {
  if (!value) {
    return null;
  }

  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : null;
}

export function to_timestamp(value?: string | null): number {
  return to_timestamp_or_null(value) ?? 0;
}
