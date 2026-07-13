import { getPickerColumnButtonClassName } from "./picker-styles";

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
          <button
            className={getPickerColumnButtonClassName(value === option, disabled)}
            disabled={disabled}
            key={option}
            onClick={() => onSelect(option)}
            type="button"
          >
            {getLabel?.(option) ?? option}
          </button>
        );
      })}
    </div>
  );
}
