/**
 * INPUT: 设置项标题、说明、选项和当前值。
 * OUTPUT: 统一的设置卡片、行、控件与响应式信息层级样式。
 * POS: 设置域共享视图原语；行级说明在窄屏仍用于解释选项影响。
 */

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export const SETTINGS_SECTION_TITLE_CLASS_NAME = cn(
  "px-1",
  getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
);
export const SETTINGS_CARD_CLASS_NAME = "overflow-hidden surface-radius-md border border-(--divider-subtle-color) bg-transparent";
export const SETTINGS_ROW_CLASS_NAME = "grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(180px,220px)] md:items-center";
export const SETTINGS_TEXT_ROW_CLASS_NAME = "flex min-w-0 items-start gap-3";
export const SETTINGS_ICON_CLASS_NAME = "flex h-7 w-7 shrink-0 items-center justify-center radius-control-sm bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary";
export const SETTINGS_ITEM_TITLE_CLASS_NAME = getUiTypographyClassName({
  role: "control",
  tone: "strong",
  weight: "semibold",
});
export const SETTINGS_ITEM_DESCRIPTION_CLASS_NAME = cn(
  "mt-1 max-w-[520px]",
  getUiTypographyClassName({ role: "supporting", tone: "soft" }),
);
export const SETTINGS_CONTROL_LABEL_CLASS_NAME = getUiTypographyClassName({
  role: "caption",
  tone: "soft",
  weight: "medium",
});
export const SETTINGS_CONTROL_HEIGHT_CLASS_NAME = "h-7";
export const SETTINGS_SELECT_BUTTON_CLASS_NAME = cn(
  SETTINGS_CONTROL_HEIGHT_CLASS_NAME,
  "w-full radius-control-md border-(--divider-subtle-color) bg-transparent px-2.5 text-(--text-strong) shadow-none hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background) focus-visible:ring-0",
  getUiTypographyClassName({ role: "caption", weight: "semibold" }),
);
