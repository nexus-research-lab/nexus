"use client";

import { type RefObject } from "react";

import { PickerPopover } from "./picker-popover";
import {
  type Meridiem,
  get_picker_column_button_class_name,
  HOUR_12_OPTIONS,
  MINUTE_OPTIONS,
  PICKER_TRIGGER_CLASS_NAME,
} from "./picker-utils";

interface DailyTimePickerProps {
  anchor_ref: RefObject<HTMLButtonElement | null>;
  display: string;
  hour12: string;
  is_open: boolean;
  meridiem: Meridiem;
  minute: string;
  on_close: () => void;
  on_hour_select: (value: string) => void;
  on_meridiem_select: (value: Meridiem) => void;
  on_minute_select: (value: string) => void;
  on_toggle: () => void;
}

export function DailyTimePicker(props: DailyTimePickerProps) {
  const {
    anchor_ref,
    display,
    hour12,
    is_open,
    meridiem,
    minute,
    on_close,
    on_hour_select,
    on_meridiem_select,
    on_minute_select,
    on_toggle,
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
        <div className="grid grid-cols-3 gap-2">
          <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
            {([{ key: "am", label: "上午" }, { key: "pm", label: "下午" }] as const).map((option) => (
              <button
                className={get_picker_column_button_class_name(meridiem === option.key)}
                key={option.key}
                onClick={() => on_meridiem_select(option.key)}
                type="button"
              >
                {option.label}
              </button>
            ))}
          </div>
          <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
            {HOUR_12_OPTIONS.map((option) => (
              <button
                className={get_picker_column_button_class_name(hour12 === option)}
                key={option}
                onClick={() => on_hour_select(option)}
                type="button"
              >
                {option}
              </button>
            ))}
          </div>
          <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
            {MINUTE_OPTIONS.map((option) => (
              <button
                className={get_picker_column_button_class_name(minute === option)}
                key={option}
                onClick={() => on_minute_select(option)}
                type="button"
              >
                {option}
              </button>
            ))}
          </div>
        </div>
      </PickerPopover>
    </div>
  );
}
