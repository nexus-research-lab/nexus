// INPUT: 图标选择器的列数与网格/横排布局语义。
// OUTPUT: 仅约束集合几何和滚动方向的稳定布局 class。
// POS: IconPicker 布局 recipe；选择状态与业务图标数据不进入本文件。

import { cn } from "@/shared/ui/class-name";

import type { IconPickerColumns, IconPickerLayout } from "./icon-picker-model";

const GRID_COLUMN_CLASS_NAMES: Record<IconPickerColumns, string> = {
  4: "grid-cols-4",
  5: "grid-cols-5",
  6: "grid-cols-6",
  8: "grid-cols-8",
};

export function getIconPickerCollectionClassName({
  columns,
  layout,
}: {
  columns: IconPickerColumns;
  layout: IconPickerLayout;
}): string {
  return layout === "row"
    ? "scrollbar-hide flex gap-2 overflow-x-auto overflow-y-hidden pb-1"
    : cn("grid gap-2", GRID_COLUMN_CLASS_NAMES[columns]);
}
