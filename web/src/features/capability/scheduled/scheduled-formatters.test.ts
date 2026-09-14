// INPUT: 同一计划协议对象与中英显示上下文。
// OUTPUT: 计划、星期和日期随当前语言投影，未知计划保持安全摘要。
// POS: 计划摘要本地化回归，不执行或改变调度。
import { expect, it } from "vitest";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { ScheduledTaskSchedule } from "@/types/capability/scheduled-task/task";
import { formatScheduledTaskSchedule } from "./scheduled-formatters";
const translate = (locale: "zh" | "en"): I18nContextValue["t"] => (key, params) =>
  Object.entries(params ?? {}).reduce((value, [name, param]) => value.replaceAll(`{${name}}`, String(param)), MESSAGES[locale][key]);
it.each([
  [{ kind: "every", interval_seconds: 1800 }, "Every 30 minutes", "每 30 分钟"],
  [{ kind: "cron", cron_expression: "30 8 * * 1,2,3,4,5", timezone: "UTC" }, "Weekdays 08:30", "工作日 08:30"],
  [{ kind: "cron", cron_expression: "0 9 15 * *", timezone: "UTC" }, "Monthly on day 15 at 09:00", "每月 15 日 09:00"],
  [{ kind: "cron", cron_expression: "*/10 * * * *", timezone: "UTC" }, "Custom schedule", "自定义计划"],
] as const)("localizes %j without changing it", (input, en, zh) => {
  const schedule = input as ScheduledTaskSchedule;
  const before = structuredClone(schedule);
  expect(formatScheduledTaskSchedule(schedule, translate("en"), "en")).toBe(en);
  expect(formatScheduledTaskSchedule(schedule, translate("zh"), "zh")).toBe(zh);
  expect(schedule).toEqual(before);
});
it("uses the requested locale for one-time dates", () => {
  const schedule: ScheduledTaskSchedule = { kind: "at", run_at: "2026-09-10T09:00:00Z" };
  const value = formatScheduledTaskSchedule(schedule, translate("en"), "en");
  expect(value).toContain("Once ·");
  expect(value).not.toMatch(/[\u4e00-\u9fff]/);
});
