import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { Locale } from "@/shared/i18n/messages";
import type { GoalCompletionReceipt } from "@/types/conversation/message/entity";

export function buildGoalCompletionReceiptItems(
  receipt: GoalCompletionReceipt,
  locale: Locale,
  t: I18nContextValue["t"],
): string[] {
  const items = [t("message.goal_completed")];
  if (Number.isFinite(receipt.time_used_seconds) && receipt.time_used_seconds! > 0) {
    items.push(t("message.goal_elapsed", {
      duration: formatGoalElapsed(receipt.time_used_seconds!, locale),
    }));
  }
  if (Number.isFinite(receipt.actual_tokens) && receipt.actual_tokens! >= 0) {
    items.push(t("message.goal_tokens_used", {
      count: Math.trunc(receipt.actual_tokens!).toLocaleString(locale === "zh" ? "zh-CN" : "en-US"),
    }));
  }
  return items;
}

export function formatGoalElapsed(secondsValue: number, locale: Locale): string {
  let remaining = Math.max(0, Math.trunc(secondsValue));
  const days = Math.floor(remaining / 86_400);
  remaining %= 86_400;
  const hours = Math.floor(remaining / 3_600);
  remaining %= 3_600;
  const minutes = Math.floor(remaining / 60);
  const seconds = remaining % 60;
  const units = locale === "zh"
    ? [[days, "天"], [hours, "小时"], [minutes, "分"], [seconds, "秒"]] as const
    : [[days, "d"], [hours, "h"], [minutes, "m"], [seconds, "s"]] as const;
  const firstNonZero = units.findIndex(([value]) => value > 0);
  return units
    .slice(firstNonZero < 0 ? units.length - 1 : firstNonZero)
    .slice(0, 2)
    .map(([value, unit]) => locale === "zh" ? `${value} ${unit}` : `${value}${unit}`)
    .join(" ");
}
