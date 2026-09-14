// INPUT: Accessible filter name, controlled options/value and a domain-owned selection command.
// OUTPUT: A compact directory filter with a value-only trigger and shared menu behavior.
// POS: Cross-domain filter pattern; owns no filtering rules, state, icons or resource queries.

import { cn } from "@/shared/ui/class-name";
import { UiSelectMenu } from "./select-menu";
import type { UiSelectMenuOption } from "./select-menu-model";

interface UiFilterSelectProps {
  ariaLabel: string;
  className?: string;
  disabled?: boolean;
  menuMinWidth?: number;
  onChange: (value: string) => void;
  options: UiSelectMenuOption[];
  placeholder?: string;
  tourAnchor?: string;
  value: string;
}

export function UiFilterSelect({
  ariaLabel, className, disabled, menuMinWidth, onChange,
  options, placeholder, tourAnchor, value,
}: UiFilterSelectProps) {
  return (
    <div className={cn("min-w-0 max-w-full shrink-0 sm:w-[176px]", className)} data-tour-anchor={tourAnchor}>
      <UiSelectMenu ariaLabel={ariaLabel} disabled={disabled} menuMinWidth={menuMinWidth}
        onChange={onChange} options={options} placeholder={placeholder} size="sm" value={value} />
    </div>
  );
}
