// INPUT: 图标族、起始编号、数量、当前值和清除开关。
// OUTPUT: 稳定的图标资源条目、选中语义与清除动作可见性。
// POS: IconPicker 纯数据模型；不返回视觉类、尺寸、颜色、阴影或布局。

import type { AvatarIconFamily } from "@/lib/avatar";

export type IconPickerColumns = 4 | 5 | 6 | 8;
export type IconPickerLayout = "grid" | "row";
export type IconPickerSize = "lg" | "md" | "sm";

interface IconPickerModelOptions {
  iconFamily: AvatarIconFamily;
  maxIcons: number;
  showClear: boolean;
  startIconId: number;
  value?: string;
}

interface IconPickerItemModel {
  active: boolean;
  iconId: string;
  iconPath: string;
  title: string;
}

interface IconPickerModel {
  items: IconPickerItemModel[];
  showClear: boolean;
}

export function buildIconPickerModel({
  iconFamily,
  maxIcons,
  showClear,
  startIconId,
  value,
}: IconPickerModelOptions): IconPickerModel {
  const iconIds = Array.from(
    { length: maxIcons },
    (_, index) => String(startIconId + index),
  );
  return {
    items: iconIds.map((iconId) => ({
      active: value === iconId,
      iconId,
      iconPath: `/icon/${iconFamily}/${iconId}.png`,
      title: `icon-${iconId}`,
    })),
    showClear: showClear && Boolean(value),
  };
}
