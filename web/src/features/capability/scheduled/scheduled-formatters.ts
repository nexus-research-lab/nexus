/**
 * =====================================================
 * @File   : scheduled-formatters.ts
 * @Date   : 2026-04-16 14:00
 * @Author : leemysw
 * 2026-04-16 14:00   Create
 * =====================================================
 */

interface FormatScheduledDatetimeOptions {
  emptyLabel?: string;
  includeSeconds?: boolean;
}

export function formatScheduledDatetime(
  value: number | null,
  options: FormatScheduledDatetimeOptions = {},
): string {
  const {
    emptyLabel = "未记录",
    includeSeconds = false,
  } = options;

  if (!value) {
    return emptyLabel;
  }

  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    ...(includeSeconds ? { second: "2-digit" as const } : {}),
  }).format(value);
}
