// INPUT: 每日时间显示值、有限时间选项、展开状态与选择命令。
// OUTPUT: 复用共享触发器、浮层和选择按钮的每日时间选择器。
// POS: Scheduled 每日/周期计划的字段组合；不拥有计划草稿或时间转换。

"use client";

import { type RefObject } from "react";

import { Clock3 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { PickerPopover } from "./picker-popover";
import { PickerTrigger } from "./picker-trigger";
import {
  HOUR_12_OPTIONS,
  MERIDIEM_OPTIONS,
  MINUTE_OPTIONS,
  type Meridiem,
} from "./picker-types";
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
  const { t } = useI18n();
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
  const pickerLabel = t("capability.scheduled_dialog_schedule");

  return (
    <div className="dialog-field">
      <PickerTrigger
        anchorRef={anchorRef}
        display={display}
        icon={Clock3}
        isOpen={isOpen}
        label={pickerLabel}
        onToggle={onToggle}
      />
      <PickerPopover
        anchorRef={anchorRef}
        ariaLabel={pickerLabel}
        isOpen={isOpen}
        onClose={onClose}
      >
        <div className="grid grid-cols-3 gap-2">
          <TimePickerColumn
            getLabel={(value) => t(value === "am"
              ? "capability.scheduled_dialog_meridiem_am"
              : "capability.scheduled_dialog_meridiem_pm")}
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
