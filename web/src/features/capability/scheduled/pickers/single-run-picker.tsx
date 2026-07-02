"use client";

import { type RefObject } from "react";

import { PickerPopover } from "./picker-popover";
import {
  HOUR_12_OPTIONS,
  MINUTE_OPTIONS,
  SECOND_OPTIONS,
  type Meridiem,
} from "./picker-types";
import {
  get_picker_column_button_class_name,
  get_picker_date_button_class_name,
  PICKER_TRIGGER_CLASS_NAME,
} from "./picker-styles";

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
    anchor_ref: anchorRef,
    display,
    hour12,
    is_date_disabled: isDateDisabled,
    is_hour_disabled: isHourDisabled,
    is_open: isOpen,
    is_meridiem_disabled: isMeridiemDisabled,
    is_minute_disabled: isMinuteDisabled,
    is_second_disabled: isSecondDisabled,
    meridiem,
    minute,
    month_label: monthLabel,
    on_close: onClose,
    on_date_select: onDateSelect,
    on_hour_select: onHourSelect,
    on_meridiem_select: onMeridiemSelect,
    on_minute_select: onMinuteSelect,
    on_next_month: onNextMonth,
    on_prev_month: onPrevMonth,
    on_second_select: onSecondSelect,
    on_toggle: onToggle,
    second,
    selected_date: selectedDate,
    visible_days: visibleDays,
  } = props;

  return (
    <div className="dialog-field">
      <button
        className={PICKER_TRIGGER_CLASS_NAME}
        onClick={onToggle}
        ref={anchorRef}
        type="button"
      >
        <span>{display}</span>
        <span className="text-xl text-(--text-default)">+</span>
      </button>
      <PickerPopover anchor_ref={anchorRef} is_open={isOpen} on_close={onClose}>
        <div className="grid gap-4 md:grid-cols-[196px,minmax(0,1fr)]">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <button className="text-sm font-semibold text-(--text-default)" onClick={onPrevMonth} type="button">上月</button>
              <span className="text-[14px] font-semibold text-(--text-strong)">{monthLabel}</span>
              <button className="text-sm font-semibold text-(--text-default)" onClick={onNextMonth} type="button">下月</button>
            </div>
            <div className="grid grid-cols-7 gap-1.5 text-center text-xs text-(--text-muted)">
              {["日", "一", "二", "三", "四", "五", "六"].map((label) => <div key={label}>{label}</div>)}
            </div>
            <div className="grid grid-cols-7 gap-1.5">
              {visibleDays.map((day) => {
                const isSelected = day.value === selectedDate;
                const isDisabled = isDateDisabled(day.value);
                return (
                  <button
                    className={get_picker_date_button_class_name(isSelected, {
                      disabled: isDisabled,
                      muted: day.muted,
                    })}
                    disabled={isDisabled}
                    key={day.value}
                    onClick={() => onDateSelect(day.value)}
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
                  const isDisabled = isMeridiemDisabled(option.key);
                  return (
                  <button
                    className={get_picker_column_button_class_name(meridiem === option.key, isDisabled)}
                    disabled={isDisabled}
                    key={option.key}
                    onClick={() => onMeridiemSelect(option.key)}
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
                  const isDisabled = isHourDisabled(option);
                  return (
                <button
                  className={get_picker_column_button_class_name(hour12 === option, isDisabled)}
                  disabled={isDisabled}
                  key={option}
                  onClick={() => onHourSelect(option)}
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
                  const isDisabled = isMinuteDisabled(option);
                  return (
                <button
                  className={get_picker_column_button_class_name(minute === option, isDisabled)}
                  disabled={isDisabled}
                  key={option}
                  onClick={() => onMinuteSelect(option)}
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
                  const isDisabled = isSecondDisabled(option);
                  return (
                <button
                  className={get_picker_column_button_class_name(second === option, isDisabled)}
                  disabled={isDisabled}
                  key={option}
                  onClick={() => onSecondSelect(option)}
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
