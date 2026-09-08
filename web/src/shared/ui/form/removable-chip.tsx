// INPUT: 可移除实体的短标签、尺寸、禁用态、可访问移除名称与命令。
// OUTPUT: 统一 chip 表面和单一原生 IconButton 移除命中区。
// POS: Form 复合输入原语；不管理实体集合、去重、排序或提交。

import type { ReactNode } from "react";
import { X } from "lucide-react";

import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

type UiRemovableChipSize = "xs" | "sm";

interface UiRemovableChipProps {
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  onRemove: () => void;
  removeLabel: string;
  size?: UiRemovableChipSize;
}

const CHIP_SIZE_CLASS_NAMES: Record<UiRemovableChipSize, string> = {
  sm: "h-7 gap-1 pl-2 pr-1",
  xs: "min-h-6 gap-1 pl-2 pr-1",
};

const CHIP_TEXT_CLASS_NAMES: Record<UiRemovableChipSize, string> = {
  sm: getUiTypographyClassName({
    role: "metadata",
    tone: "default",
    weight: "medium",
  }),
  xs: getUiTypographyClassName({
    role: "caption",
    tone: "strong",
    weight: "medium",
  }),
};

const CHIP_ICON_CLASS_NAMES: Record<UiRemovableChipSize, string> = {
  sm: "h-3 w-3",
  xs: "h-2.5 w-2.5",
};

export function UiRemovableChip({
  children,
  className,
  disabled = false,
  onRemove,
  removeLabel,
  size = "sm",
}: UiRemovableChipProps) {
  return (
    <span
      className={cn(
        "chip-default inline-flex max-w-full shrink-0 items-center",
        CHIP_SIZE_CLASS_NAMES[size],
        disabled && "opacity-(--disabled-opacity)",
        className,
      )}
      data-disabled={disabled || undefined}
    >
      <span className={cn("min-w-0 truncate", CHIP_TEXT_CLASS_NAMES[size])}>
        {children}
      </span>
      <UiIconButton
        aria-label={removeLabel}
        className="pointer-events-auto shrink-0 text-(--icon-muted)"
        disabled={disabled}
        onClick={onRemove}
        size="2xs"
        variant="ghost"
      >
        <X className={CHIP_ICON_CLASS_NAMES[size]} />
      </UiIconButton>
    </span>
  );
}
