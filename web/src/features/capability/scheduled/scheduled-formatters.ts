/**
 * =====================================================
 * @File   : scheduled-formatters.ts
 * @Date   : 2026-04-16 14:00
 * @Author : leemysw
 * 2026-04-16 14:00   Create
 * =====================================================
 */

interface FormatScheduledDatetimeOptions {
  empty_label?: string;
  include_seconds?: boolean;
}

export function format_scheduled_datetime(
  value: number | null,
  options: FormatScheduledDatetimeOptions = {},
): string {
  const {
    empty_label = "未记录",
    include_seconds = false,
  } = options;

  if (!value) {
    return empty_label;
  }

  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    ...(include_seconds ? { second: "2-digit" as const } : {}),
  }).format(value);
}
