// INPUT: 最近 365 个 UTC 自然日的真实 Token 汇总。
// OUTPUT: 可键盘查看的年度热力图、堆叠柱状图和精确明细表。
// POS: 个人用量趋势视图，不补造旧服务未返回的历史。
import { useState } from "react";
import type { DailyTokenUsage } from "@/lib/api/account/auth-api";
import { formatTokens } from "@/lib/format/token-count";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { SETTINGS_CARD_CLASS_NAME } from "../shared/settings-panel-ui";

export function PersonalUsageHistory({ days: allDays }: { days: DailyTokenUsage[] }) {
  const { locale, t } = useI18n();
  const [view, setView] = useState<"chart" | "table">("chart");
  const days = allDays.slice(-30);
  const [selected, setSelected] = useState<string | null>(null);
  const active = days.find((day) => day.date === selected) ?? days.at(-1);
  const series = [
    { key: "input_tokens", label: t("settings.personal.input_tokens"), color: "bg-primary" },
    { key: "output_tokens", label: t("settings.personal.output_tokens"), color: "bg-(--accent)" },
    { key: "cache_tokens", label: t("settings.personal.cache_tokens"), color: "bg-(--text-muted)" },
  ] as const;
  const maximum = Math.max(1, ...days.map((day) => day.input_tokens + day.output_tokens + day.cache_tokens));
  return <section className={`${SETTINGS_CARD_CLASS_NAME} space-y-5 p-5`}>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div><h3 className="ui-type-section-title">{t("settings.personal.daily_usage")}</h3>
        <p className="ui-type-metadata mt-1 text-(--text-muted)">{t("settings.personal.daily_period")}</p></div>
      <div className="flex flex-wrap items-center gap-2">
      <UiSegmentedControl title={t("settings.personal.daily_usage")} value={view} onChange={setView} density="compact"
        options={[{ value: "chart", label: t("settings.personal.chart") }, { value: "table", label: t("settings.personal.table") }]} />
      </div>
    </div>
    <div className="ui-type-metadata flex flex-wrap gap-4 text-(--text-muted)">{series.map((item) => <span key={item.key} className="flex items-center gap-2"><span className={`h-2 w-2 rounded-sm ${item.color}`} />{item.label}</span>)}</div>
    {view === "chart" ? <>
      <div className="flex gap-3">
        <div aria-hidden="true" className="ui-type-caption flex h-48 flex-col justify-between text-right tabular-nums text-(--text-muted)"><span>{formatTokens(maximum, locale)}</span><span>{formatTokens(maximum / 2, locale)}</span><span>0</span></div>
        <div className="flex h-48 min-w-0 flex-1 items-end gap-1 border-b border-(--divider-subtle-color) bg-[linear-gradient(to_top,var(--divider-subtle-color)_1px,transparent_1px)] bg-size-[100%_50%]">
          {days.map((day) => <UiButton key={day.date} aria-label={`${day.date}: ${day.total_tokens.toLocaleString()}`} aria-pressed={active?.date === day.date}
            className="h-full min-h-0 min-w-0 flex-1 flex-col justify-end gap-0 rounded-none border-0 p-0" variant="ghost"
            onMouseEnter={() => setSelected(day.date)} onFocus={() => setSelected(day.date)} onClick={() => setSelected(day.date)}>
            {series.map((item) => <span aria-hidden="true" key={item.key} className={`block w-full shrink-0 ${item.color}`} style={{ height: `${day[item.key] / maximum * 100}%` }} />)}
          </UiButton>)}
        </div>
      </div>
      <div className="ui-type-caption flex justify-between text-(--text-muted)"><span>{days[0]?.date}</span><span>{days.at(-1)?.date}</span></div>
      <div aria-live="polite" className="ui-type-metadata flex min-h-10 flex-wrap items-center gap-x-4 gap-y-1 tabular-nums">
        <span>{active?.date}</span>{series.map((item) => <span key={item.key}>{item.label} · {formatTokens(active?.[item.key] ?? 0, locale)}</span>)}
      </div>
    </> : <div className="max-h-72 overflow-auto"><table className="ui-type-metadata w-full text-right tabular-nums"><thead><tr><th className="sticky top-0 bg-(--background) py-2 text-left">{t("settings.personal.date")}</th>{series.map((item) => <th className="sticky top-0 bg-(--background) px-2 py-2 font-medium" key={item.key}>{item.label}</th>)}</tr></thead><tbody>{days.toReversed().map((day) => <tr className="border-t border-(--divider-subtle-color)" key={day.date}><td className="py-2 text-left">{day.date}</td>{series.map((item) => <td className="px-2 py-2" key={item.key}>{day[item.key].toLocaleString()}</td>)}</tr>)}</tbody></table></div>}
  </section>;
}

// 日历范围不依赖接口返回条数；旧服务只返回 30 天时，仍保留全年空格。
function buildUsageCalendar(days: DailyTokenUsage[]) {
  const end = new Date();
  const values = new Map(days.map((day) => [day.date, day.total_tokens]));
  return Array.from({ length: 365 }, (_, index) => {
    const date = new Date(Date.UTC(end.getUTCFullYear(), end.getUTCMonth(), end.getUTCDate() - 364 + index))
      .toISOString().slice(0, 10);
    return { date, value: values.get(date) };
  });
}

export function PersonalUsageHeatmap({ days }: { days: DailyTokenUsage[] }) {
  const { locale, t } = useI18n();
  const [mode, setMode] = useState<"daily" | "weekly" | "cumulative">("daily");
  const calendar = buildUsageCalendar(days);
  const offset = new Date(`${calendar[0].date}T00:00:00Z`).getUTCDay();
  const columns = Math.ceil((offset + calendar.length) / 7);
  const weeks = new Map<number, number>();
  calendar.forEach((day, index) => {
    if (day.value === undefined) return;
    const week = Math.floor((offset + index) / 7);
    weeks.set(week, (weeks.get(week) ?? 0) + day.value);
  });
  let cumulative = 0;
  const cells = calendar.map((day, index) => {
    cumulative += day.value ?? 0;
    const values = { daily: day.value, weekly: weeks.get(Math.floor((offset + index) / 7)), cumulative };
    return { ...day, value: day.value === undefined ? undefined : values[mode] };
  });
  const peak = Math.max(1, ...cells.map((day) => day.value ?? 0));
  const months = calendar.flatMap((day, index) => day.date.endsWith("-01")
    ? [{ date: day.date, column: Math.floor((offset + index) / 7) + 1 }] : []);
  return <section aria-label={t("settings.personal.token_activity")} className="space-y-2 py-1">
    <div className="flex flex-wrap items-baseline justify-between gap-2">
      <h3 className="ui-type-section-title">{t("settings.personal.token_activity")}</h3>
      <UiSegmentedControl title={t("settings.personal.token_activity")} value={mode} onChange={setMode} density="compact"
        options={[{ value: "daily", label: t("settings.personal.activity_daily") }, { value: "weekly", label: t("settings.personal.activity_weekly") }, { value: "cumulative", label: t("settings.personal.activity_cumulative") }]} />
    </div>

      <div className="overflow-x-auto pb-2">
        <div className="min-w-[640px]">
        <div data-usage-calendar className="grid grid-flow-col gap-[3px]" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`, gridTemplateRows: "repeat(7, auto)" }}>
          {Array.from({length: offset}, (_, index) => <span key={`pad-${index}`} />)}
          {cells.map((day) => {
            const strength = day.value ? 0.25 + Math.ceil(day.value / peak * 3) * 0.25 : 0;
            const label = `${day.date}: ${day.value === undefined ? t("settings.personal.activity_missing") : `${day.value.toLocaleString()} Token`}`;
            const dateLabel = new Date(`${day.date}T00:00:00Z`).toLocaleDateString(locale, { month: "short", day: "numeric", timeZone: "UTC" });
            const detail = day.value === undefined ? `${dateLabel} · ${t("settings.personal.activity_missing")}` : t("settings.personal.activity_detail", { date: dateLabel, value: formatTokens(day.value, locale) });
            return <UiTooltip key={day.date} label={detail}><UiButton aria-label={label}
              className="aspect-square h-auto min-h-0 w-full min-w-0 rounded-sm border-0 p-0" variant="ghost"
              style={{backgroundColor: strength === 0 ? "var(--surface-interactive-hover-background)" : `color-mix(in srgb, var(--primary) ${strength * 100}%, var(--background))`}}
            >{null}</UiButton></UiTooltip>;
          })}
        </div>
        <div className="ui-type-caption mt-1 grid gap-[3px] text-(--text-muted)" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
          {months.map((month) => <span key={month.date} className="whitespace-nowrap" style={{ gridColumn: month.column }}>
            {new Date(`${month.date}T00:00:00Z`).toLocaleDateString(locale, { month: "short", timeZone: "UTC" })}
          </span>)}
        </div>
        </div>
      </div>
      <p className="ui-type-caption text-(--text-muted)">{t("settings.personal.heatmap_period")}</p>
  </section>;
}
