"use client";

import { type RefObject } from "react";

import { PickerPopover } from "./picker-popover";
import {
  type Meridiem,
  get_picker_column_button_class_name,
  HOUR_12_OPTIONS,
  MINUTE_OPTIONS,
  PICKER_TRIGGER_CLASS_NAME,
  SECOND_OPTIONS,
} from "./picker-utils";

interface CalendarDay {
  label: string;
  muted: boolean;
  value: string;
}

interface SingleRunPickerProps {
  anchor_ref: RefObject<HTMLButtonElement | null>;
  display: string;
  hour12: string;
  is_date_disabled: (value: string) => boolean;
  is_hour_disabled: (value: string) => boolean;
  is_open: boolean;
  is_meridiem_disabled: (value: Meridiem) => boolean;
  is_minute_disabled: (value: string) => boolean;
  is_second_disabled: (value: string) => boolean;
  meridiem: Meridiem;
  minute: string;
  month_label: string;
  on_close: () => void;
  on_date_select: (value: string) => void;
  on_hour_select: (value: string) => void;
  on_meridiem_select: (value: Meridiem) => void;
  on_minute_select: (value: string) => void;
  on_next_month: () => void;
  on_prev_month: () => void;
  on_second_select: (value: string) => void;
  on_toggle: () => void;
  second: string;
  selected_date: string;
  visible_days: CalendarDay[];
}

export function SingleRunPicker(props: SingleRunPickerProps) {
  const {
    anchor_ref,
    display,
    hour12,
    is_date_disabled,
    is_hour_disabled,
    is_open,
    is_meridiem_disabled,
    is_minute_disabled,
    is_second_disabled,
    meridiem,
    minute,
    month_label,
    on_close,
    on_date_select,
    on_hour_select,
    on_meridiem_select,
    on_minute_select,
    on_next_month,
    on_prev_month,
    on_second_select,
    on_toggle,
    second,
    selected_date,
    visible_days,
  } = props;

  return (
    <div className="dialog-field">
      <button
        className={PICKER_TRIGGER_CLASS_NAME}
        onClick={on_toggle}
        ref={anchor_ref}
        type="button"
      >
        <span>{display}</span>
        <span className="text-xl text-(--text-default)">+</span>
      </button>
      <PickerPopover anchor_ref={anchor_ref} is_open={is_open} on_close={on_close}>
        <div className="grid gap-4 md:grid-cols-[196px,minmax(0,1fr)]">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <button className="text-sm font-semibold text-(--text-default)" onClick={on_prev_month} type="button">上月</button>
              <span className="text-[14px] font-semibold text-(--text-strong)">{month_label}</span>
              <button className="text-sm font-semibold text-(--text-default)" onClick={on_next_month} type="button">下月</button>
            </div>
            <div className="grid grid-cols-7 gap-1.5 text-center text-xs text-(--text-muted)">
              {["日", "一", "二", "三", "四", "五", "六"].map((label) => <div key={label}>{label}</div>)}
            </div>
            <div className="grid grid-cols-7 gap-1.5">
              {visible_days.map((day) => {
                const is_selected = day.value === selected_date;
                const is_disabled = is_date_disabled(day.value);
                return (
                  <button
                    className={[
                      "flex h-8 items-center justify-center rounded-[10px] text-xs font-semibold transition-[background,color] duration-(--motion-duration-fast)",
                      is_selected
                        ? "bg-[color:color-mix(in_srgb,var(--primary)_90%,white)] text-white"
                        : is_disabled
                          ? "cursor-not-allowed text-(--text-soft) opacity-40"
                          : day.muted
                              ? "text-(--text-soft)"
                              : "text-(--text-default) hover:bg-(--surface-interactive-hover-background)",
                    ].join(" ")}
                    disabled={is_disabled}
                    key={day.value}
                    onClick={() => on_date_select(day.value)}
                    type="button"
                  >
                    {day.label}
                  </button>
                );
              })}
            </div>
          </div>
          <div className="grid grid-cols-4 gap-2">
            <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
              {([{ key: "am", label: "上午" }, { key: "pm", label: "下午" }] as const).map((option) => (
                (() => {
                  const is_disabled = is_meridiem_disabled(option.key);
                  return (
                  <button
                    className={get_picker_column_button_class_name(meridiem === option.key)}
                    disabled={is_disabled}
                    key={option.key}
                    onClick={() => on_meridiem_select(option.key)}
                    type="button"
                  >
                    {option.label}
                  </button>
                  );
                })()
              ))}
            </div>
            <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
              {HOUR_12_OPTIONS.map((option) => (
                (() => {
                  const is_disabled = is_hour_disabled(option);
                  return (
                <button
                  className={get_picker_column_button_class_name(hour12 === option)}
                  disabled={is_disabled}
                  key={option}
                  onClick={() => on_hour_select(option)}
                  type="button"
                >
                  {option}
                </button>
                  );
                })()
              ))}
            </div>
            <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
              {MINUTE_OPTIONS.map((option) => (
                (() => {
                  const is_disabled = is_minute_disabled(option);
                  return (
                <button
                  className={get_picker_column_button_class_name(minute === option)}
                  disabled={is_disabled}
                  key={option}
                  onClick={() => on_minute_select(option)}
                  type="button"
                >
                  {option}
                </button>
                  );
                })()
              ))}
            </div>
            <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
              {SECOND_OPTIONS.map((option) => (
                (() => {
                  const is_disabled = is_second_disabled(option);
                  return (
                <button
                  className={get_picker_column_button_class_name(second === option)}
                  disabled={is_disabled}
                  key={option}
                  onClick={() => on_second_select(option)}
                  type="button"
                >
                  {option}
                </button>
                  );
                })()
              ))}
            </div>
          </div>
        </div>
      </PickerPopover>
    </div>
  );
}
