"use client";

import { type RefObject } from "react";

import { PickerPopover } from "./picker-popover";
import {
  HOUR_12_OPTIONS,
  MERIDIEM_LABELS,
  MERIDIEM_OPTIONS,
  MINUTE_OPTIONS,
  type Meridiem,
} from "./picker-types";
import {
  PICKER_TRIGGER_CLASS_NAME,
} from "./picker-styles";
import { TimePickerColumn } from "./time-picker-column";

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
    anchorRef,
    display,
    hour12,
    isOpen,
    meridiem,
    minute,
    onClose,
    onHourSelect,
    onMeridiemSelect,
    onMinuteSelect,
    onToggle,
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
          <TimePickerColumn
            getLabel={(value) => MERIDIEM_LABELS[value]}
            onSelect={onMeridiemSelect}
            options={MERIDIEM_OPTIONS}
            value={meridiem}
          />
          <TimePickerColumn
            onSelect={onHourSelect}
            options={HOUR_12_OPTIONS}
            value={hour12}
          />
          <TimePickerColumn
            onSelect={onMinuteSelect}
            options={MINUTE_OPTIONS}
            value={minute}
          />
        </div>
      </PickerPopover>
    </div>
  );
}
