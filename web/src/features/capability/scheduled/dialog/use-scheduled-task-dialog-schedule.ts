"use client";

import { useCallback, useState } from "react";

import {
  build_calendar_days,
  build_datetime_local_input,
  build_time_value,
  format_datetime_display,
  format_datetime_local_input,
  format_time_display,
  format_time_local_input,
  from_meridiem_parts,
  split_datetime_local_input,
  split_time_value,
  to_meridiem_parts,
  type Meridiem,
  type Weekday,
} from "../pickers/picker-utils";
import {
  zonedDateTimeToEpochMs,
  type EveryUnit,
  type ScheduleKind,
} from "./scheduled-task-dialog-constants";

export function useScheduledTaskDialogScheduleState(timezone: string) {
  const now = new Date();
  const now_date = `${now.getFullYear()}-${`${now.getMonth() + 1}`.padStart(2, "0")}-${`${now.getDate()}`.padStart(2, "0")}`;
  const [schedule_kind, set_schedule_kind] = useState<ScheduleKind>("every");
  const [every_value, set_every_value] = useState("30");
  const [every_unit, set_every_unit] = useState<EveryUnit>("minutes");
  const [daily_time, set_daily_time] = useState(format_time_local_input(new Date(Date.now() + 3600_000)));
  const [selected_weekdays, set_selected_weekdays] = useState<Weekday[]>(["mo", "tu", "we", "th", "fr", "sa", "su"]);
  const [run_at, set_run_at] = useState(format_datetime_local_input(new Date(Date.now() + 3600_000)));
  const [is_daily_picker_open, set_is_daily_picker_open] = useState(false);
  const [is_single_picker_open, set_is_single_picker_open] = useState(false);
  const [single_picker_month, set_single_picker_month] = useState(format_datetime_local_input(new Date(Date.now())).slice(0, 7));

  const daily_time_parts = split_time_value(daily_time);
  const run_at_parts = split_datetime_local_input(run_at);
  const daily_meridiem_parts = to_meridiem_parts(daily_time_parts.hour, daily_time_parts.minute);
  const single_meridiem_parts = to_meridiem_parts(run_at_parts.hour, run_at_parts.minute, run_at_parts.second);
  const single_picker_days = build_calendar_days(single_picker_month);

  const reset = useCallback(() => {
    set_schedule_kind("every");
    set_every_value("30");
    set_every_unit("minutes");
    set_daily_time(format_time_local_input(new Date(Date.now() + 3600_000)));
    set_selected_weekdays(["mo", "tu", "we", "th", "fr", "sa", "su"]);
    set_run_at(format_datetime_local_input(new Date(Date.now() + 3600_000)));
    set_is_daily_picker_open(false);
    set_is_single_picker_open(false);
    set_single_picker_month(format_datetime_local_input(new Date(Date.now() + 3600_000)).slice(0, 7));
  }, []);

  const hydrate = useCallback((params: {
    schedule_kind: ScheduleKind;
    every_value?: string;
    every_unit?: EveryUnit;
    daily_time?: string;
    selected_weekdays?: Weekday[];
    run_at?: string;
  }) => {
    set_schedule_kind(params.schedule_kind);
    set_every_value(params.every_value ?? "30");
    set_every_unit(params.every_unit ?? "minutes");
    set_daily_time(params.daily_time ?? format_time_local_input(new Date(Date.now() + 3600_000)));
    set_selected_weekdays(params.selected_weekdays ?? ["mo", "tu", "we", "th", "fr", "sa", "su"]);
    const next_run_at = params.run_at ?? format_datetime_local_input(new Date(Date.now() + 3600_000));
    set_run_at(next_run_at);
    set_is_daily_picker_open(false);
    set_is_single_picker_open(false);
    set_single_picker_month(next_run_at.slice(0, 7));
  }, []);

  function update_daily_picker(next: { meridiem?: Meridiem; hour12?: string; minute?: string }) {
    const merged = {
      meridiem: next.meridiem ?? daily_meridiem_parts.meridiem,
      hour12: next.hour12 ?? daily_meridiem_parts.hour12,
      minute: next.minute ?? daily_meridiem_parts.minute,
    };
    const converted = from_meridiem_parts(merged.meridiem, merged.hour12, merged.minute);
    set_daily_time(build_time_value(converted.hour24, converted.minute));
  }

  function update_single_picker(next: { date?: string; meridiem?: Meridiem; hour12?: string; minute?: string; second?: string }) {
    const merged = {
      date: next.date ?? run_at_parts.date,
      meridiem: next.meridiem ?? single_meridiem_parts.meridiem,
      hour12: next.hour12 ?? single_meridiem_parts.hour12,
      minute: next.minute ?? single_meridiem_parts.minute,
      second: next.second ?? single_meridiem_parts.second,
    };
    const converted = from_meridiem_parts(merged.meridiem, merged.hour12, merged.minute, merged.second);
    set_run_at(build_datetime_local_input(merged.date, converted.hour24, converted.minute, converted.second));
  }

  function toggle_weekday(weekday: Weekday) {
    set_selected_weekdays((current) =>
      current.includes(weekday) ? current.filter((item) => item !== weekday) : [...current, weekday],
    );
  }

  function go_to_prev_month() {
    const [year, month] = single_picker_month.split("-").map(Number);
    const prev = new Date(year, month - 2, 1);
    set_single_picker_month(`${prev.getFullYear()}-${`${prev.getMonth() + 1}`.padStart(2, "0")}`);
  }

  function go_to_next_month() {
    const [year, month] = single_picker_month.split("-").map(Number);
    const next = new Date(year, month, 1);
    set_single_picker_month(`${next.getFullYear()}-${`${next.getMonth() + 1}`.padStart(2, "0")}`);
  }

  function sync_single_picker_to_now() {
    const now_value = new Date();
    set_run_at(format_datetime_local_input(now_value));
    set_single_picker_month(format_datetime_local_input(now_value).slice(0, 7));
  }

  function build_single_candidate_input(params: {
    date?: string;
    meridiem?: Meridiem;
    hour12?: string;
    minute?: string;
    second?: string;
  }): string {
    const merged = {
      date: params.date ?? run_at_parts.date,
      meridiem: params.meridiem ?? single_meridiem_parts.meridiem,
      hour12: params.hour12 ?? single_meridiem_parts.hour12,
      minute: params.minute ?? single_meridiem_parts.minute,
      second: params.second ?? single_meridiem_parts.second,
    };
    const converted = from_meridiem_parts(merged.meridiem, merged.hour12, merged.minute, merged.second);
    return build_datetime_local_input(merged.date, converted.hour24, converted.minute, converted.second);
  }

  function is_single_date_disabled(date_value: string): boolean {
    const epoch_ms = zonedDateTimeToEpochMs(build_single_candidate_input({ date: date_value }), timezone);
    return epoch_ms !== null && epoch_ms <= Date.now();
  }

  function is_single_meridiem_disabled(value: Meridiem): boolean {
    const epoch_ms = zonedDateTimeToEpochMs(build_single_candidate_input({ meridiem: value }), timezone);
    return epoch_ms !== null && epoch_ms <= Date.now();
  }

  function is_single_hour_disabled(value: string): boolean {
    const epoch_ms = zonedDateTimeToEpochMs(build_single_candidate_input({ hour12: value }), timezone);
    return epoch_ms !== null && epoch_ms <= Date.now();
  }

  function is_single_minute_disabled(value: string): boolean {
    const epoch_ms = zonedDateTimeToEpochMs(build_single_candidate_input({ minute: value }), timezone);
    return epoch_ms !== null && epoch_ms <= Date.now();
  }

  function is_single_second_disabled(value: string): boolean {
    const epoch_ms = zonedDateTimeToEpochMs(build_single_candidate_input({ second: value }), timezone);
    return epoch_ms !== null && epoch_ms <= Date.now();
  }

  return {
    schedule_kind,
    set_schedule_kind,
    every_value,
    set_every_value,
    every_unit,
    set_every_unit,
    daily_time,
    selected_weekdays,
    set_selected_weekdays,
    run_at,
    set_run_at,
    is_daily_picker_open,
    set_is_daily_picker_open,
    is_single_picker_open,
    set_is_single_picker_open,
    single_picker_month,
    set_single_picker_month,
    daily_time_parts,
    run_at_parts,
    daily_meridiem_parts,
    single_meridiem_parts,
    single_picker_days,
    daily_display: format_time_display(daily_time_parts.hour, daily_time_parts.minute),
    run_at_display: format_datetime_display(run_at_parts.date, run_at_parts.hour, run_at_parts.minute, run_at_parts.second),
    update_daily_picker,
    update_single_picker,
    toggle_weekday,
    go_to_prev_month,
    go_to_next_month,
    sync_single_picker_to_now,
    now_date,
    is_single_date_disabled,
    is_single_meridiem_disabled,
    is_single_hour_disabled,
    is_single_minute_disabled,
    is_single_second_disabled,
    reset,
    hydrate,
  };
}
