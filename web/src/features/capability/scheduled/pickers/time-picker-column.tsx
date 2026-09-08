// INPUT: 有序时间选项、当前值、可选禁用规则与选择命令。
// OUTPUT: 使用共享 ChoiceButton 的单列时间选项。
// POS: Scheduled Picker 的无状态选项列；不管理滚动位置或时间转换。

import { UiChoiceButton } from "@/shared/ui/form/choice";

interface TimePickerColumnProps<T extends string> {
  getLabel?: (value: T) => string;
  isDisabled?: (value: T) => boolean;
  onSelect: (value: T) => void;
  options: readonly T[];
  value: T;
}

export function TimePickerColumn<T extends string>({
  getLabel,
  isDisabled,
  onSelect,
  options,
  value,
}: TimePickerColumnProps<T>) {
  return (
    <div className="max-h-[240px] space-y-2 overflow-y-auto pr-1">
      {options.map((option) => {
        const disabled = isDisabled?.(option) ?? false;
        return (
          <UiChoiceButton
            active={value === option}
            className="w-full"
            disabled={disabled}
            key={option}
            onClick={() => onSelect(option)}
            variant="picker"
          >
            {getLabel?.(option) ?? option}
          </UiChoiceButton>
        );
      })}
    </div>
  );
}
