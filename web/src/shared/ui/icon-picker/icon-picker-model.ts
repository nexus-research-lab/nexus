import type { AvatarIconFamily } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";

export type IconPickerColumns = 4 | 6 | 8;
export type IconPickerLayout = "grid" | "row";
export type IconPickerSize = "lg" | "md" | "sm";

interface IconPickerPresentationOptions {
  columns: IconPickerColumns;
  disabled: boolean;
  iconFamily: AvatarIconFamily;
  iconSize: IconPickerSize;
  layout: IconPickerLayout;
  maxIcons: number;
  showClear: boolean;
  startIconId: number;
  value?: string;
}

interface IconPickerItemPresentation {
  className: string;
  iconId: string;
  iconPath: string;
  title: string;
}

interface IconPickerPresentation {
  collectionClassName: string;
  items: IconPickerItemPresentation[];
  showClear: boolean;
}

const GRID_COLUMN_CLASS_NAMES: Record<IconPickerColumns, string> = {
  4: "grid-cols-4",
  6: "grid-cols-6",
  8: "grid-cols-8",
};

const ICON_SIZE_CLASS_NAMES: Record<IconPickerSize, string> = {
  lg: "h-12 w-12",
  md: "h-10 w-10",
  sm: "h-8 w-8",
};

const ICON_STATE_CLASS_NAMES = {
  idle: "border border-(--surface-inset-border) bg-transparent hover:bg-(--surface-interactive-hover-background)",
  selected: "bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] border border-(--primary) shadow-[0_0_0_1px_color-mix(in_srgb,var(--primary)_16%,transparent)]",
} as const;

function buildIconPickerItem(
  iconId: string,
  options: IconPickerPresentationOptions,
): IconPickerItemPresentation {
  const state = options.value === iconId ? "selected" : "idle";
  return {
    className: cn(
      "relative inline-flex cursor-pointer items-center justify-center overflow-hidden rounded-[12px] transition-[background,border-color,box-shadow] duration-(--motion-duration-fast)",
      ICON_SIZE_CLASS_NAMES[options.iconSize],
      options.layout === "row" && "shrink-0",
      ICON_STATE_CLASS_NAMES[state],
      options.disabled && "cursor-not-allowed opacity-50",
    ),
    iconId,
    iconPath: `/icon/${options.iconFamily}/${iconId}.png`,
    title: `icon-${iconId}`,
  };
}

export function getIconPickerPresentation(
  options: IconPickerPresentationOptions,
): IconPickerPresentation {
  const iconIds = Array.from(
    { length: options.maxIcons },
    (_, index) => String(options.startIconId + index),
  );
  return {
    collectionClassName: options.layout === "row"
      ? "soft-scrollbar flex gap-2 overflow-x-auto overflow-y-hidden pb-1"
      : cn("grid gap-2", GRID_COLUMN_CLASS_NAMES[options.columns]),
    items: iconIds.map((iconId) => buildIconPickerItem(iconId, options)),
    showClear: options.showClear && Boolean(options.value),
  };
}
