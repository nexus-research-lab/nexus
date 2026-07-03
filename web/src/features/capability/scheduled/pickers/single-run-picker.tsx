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
  getPickerColumnButtonClassName,
  getPickerDateButtonClassName,
  PICKER_TRIGGER_CLASS_NAME,
} from "./picker-styles";

interface CalendarDay {
  label: string;
  muted: boolean;
  value: string;
}

interface SingleRunPickerProps {
  anchorRef: RefObject<HTMLButtonElement | null>;
  display: string;
  hour12: string;
  isDateDisabled: (value: string) => boolean;
  isHourDisabled: (value: string) => boolean;
  isOpen: boolean;
  isMeridiemDisabled: (value: Meridiem) => boolean;
  isMinuteDisabled: (value: string) => boolean;
  isSecondDisabled: (value: string) => boolean;
  meridiem: Meridiem;
  minute: string;
  monthLabel: string;
  onClose: () => void;
  onDateSelect: (value: string) => void;
  onHourSelect: (value: string) => void;
  onMeridiemSelect: (value: Meridiem) => void;
  onMinuteSelect: (value: string) => void;
  onNextMonth: () => void;
  onPrevMonth: () => void;
  onSecondSelect: (value: string) => void;
  onToggle: () => void;
  second: string;
  selectedDate: string;
  visibleDays: CalendarDay[];
}

export function SingleRunPicker(props: SingleRunPickerProps) {
  const {
    anchorRef: anchorRef,
    display,
    hour12,
    isDateDisabled: isDateDisabled,
    isHourDisabled: isHourDisabled,
    isOpen: isOpen,
    isMeridiemDisabled: isMeridiemDisabled,
    isMinuteDisabled: isMinuteDisabled,
    isSecondDisabled: isSecondDisabled,
    meridiem,
    minute,
    monthLabel: monthLabel,
    onClose: onClose,
    onDateSelect: onDateSelect,
    onHourSelect: onHourSelect,
    onMeridiemSelect: onMeridiemSelect,
    onMinuteSelect: onMinuteSelect,
    onNextMonth: onNextMonth,
    onPrevMonth: onPrevMonth,
    onSecondSelect: onSecondSelect,
    onToggle: onToggle,
    second,
    selectedDate: selectedDate,
    visibleDays: visibleDays,
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
      <PickerPopover anchorRef={anchorRef} isOpen={isOpen} onClose={onClose}>
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
                    className={getPickerDateButtonClassName(isSelected, {
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
                    className={getPickerColumnButtonClassName(meridiem === option.key, isDisabled)}
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
                  className={getPickerColumnButtonClassName(hour12 === option, isDisabled)}
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
                  className={getPickerColumnButtonClassName(minute === option, isDisabled)}
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
                  className={getPickerColumnButtonClassName(second === option, isDisabled)}
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
