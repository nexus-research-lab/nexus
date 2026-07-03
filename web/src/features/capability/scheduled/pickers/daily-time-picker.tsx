"use client";

import { type RefObject } from "react";

import { PickerPopover } from "./picker-popover";
import {
  HOUR_12_OPTIONS,
  MINUTE_OPTIONS,
  type Meridiem,
} from "./picker-types";
import {
  getPickerColumnButtonClassName,
  PICKER_TRIGGER_CLASS_NAME,
} from "./picker-styles";

interface DailyTimePickerProps {
  anchorRef: RefObject<HTMLButtonElement | null>;
  display: string;
  hour12: string;
  isOpen: boolean;
  meridiem: Meridiem;
  minute: string;
  onClose: () => void;
  onHourSelect: (value: string) => void;
  onMeridiemSelect: (value: Meridiem) => void;
  onMinuteSelect: (value: string) => void;
  onToggle: () => void;
}

export function DailyTimePicker(props: DailyTimePickerProps) {
  const {
    anchorRef: anchorRef,
    display,
    hour12,
    isOpen: isOpen,
    meridiem,
    minute,
    onClose: onClose,
    onHourSelect: onHourSelect,
    onMeridiemSelect: onMeridiemSelect,
    onMinuteSelect: onMinuteSelect,
    onToggle: onToggle,
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
        <div className="grid grid-cols-3 gap-2">
          <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
            {([{ key: "am", label: "上午" }, { key: "pm", label: "下午" }] as const).map((option) => (
              <button
                className={getPickerColumnButtonClassName(meridiem === option.key)}
                key={option.key}
                onClick={() => onMeridiemSelect(option.key)}
                type="button"
              >
                {option.label}
              </button>
            ))}
          </div>
          <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
            {HOUR_12_OPTIONS.map((option) => (
              <button
                className={getPickerColumnButtonClassName(hour12 === option)}
                key={option}
                onClick={() => onHourSelect(option)}
                type="button"
              >
                {option}
              </button>
            ))}
          </div>
          <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
            {MINUTE_OPTIONS.map((option) => (
              <button
                className={getPickerColumnButtonClassName(minute === option)}
                key={option}
                onClick={() => onMinuteSelect(option)}
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
