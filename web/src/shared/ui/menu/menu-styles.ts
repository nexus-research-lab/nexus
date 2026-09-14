// INPUT: 菜单条目密度、说明、行高/分隔线数量、active 与 default/primary/danger tone。
// OUTPUT: 菜单共用的语义排版、固定/自适应行最小尺寸、总高、分隔与排除禁用态的视觉反馈。
// POS: Menu 视觉合同；不渲染 DOM、定位浮层或持有业务选值。

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export type UiMenuItemTone = "default" | "primary" | "danger";
export type UiMenuItemDensity = "compact" | "default";

const MENU_ITEM_LAYOUT = {
  compact: {
    described: { fixed: "h-11", minimum: "min-h-11", spacing: "gap-2 px-2 py-0.5", height: 44 },
    plain: { fixed: "h-8", minimum: "min-h-8", spacing: "gap-2 px-2 py-0.5", height: 32 },
  },
  default: {
    described: { fixed: "h-12", minimum: "min-h-12", spacing: "gap-3 px-2.5 py-1", height: 48 },
    plain: { fixed: "h-9", minimum: "min-h-9", spacing: "gap-3 px-2.5 py-1", height: 36 },
  },
} as const;

/** 固定建议行与随内容增长的动作行共用最小尺寸；实际长文本高度由浮层测量。 */
export function getMenuItemLayout({ density = "default", hasDescription = false, contentSized = false }: {
  density?: UiMenuItemDensity;
  hasDescription?: boolean;
  contentSized?: boolean;
} = {}) {
  const layout = MENU_ITEM_LAYOUT[density][hasDescription ? "described" : "plain"];
  return {
    height: layout.height,
    className: cn(contentSized ? layout.minimum : layout.fixed, layout.spacing,
      getUiTypographyClassName({ role: density === "compact" ? "supporting" : "control", weight: "regular" })),
  };
}

/** 菜单型浮层统一使用 4px 外边距和 2px 条目节奏。 */
export const MENU_LIST_CLASS_NAME = "flex flex-col gap-0.5";
export const MENU_ITEM_GAP_PX = 2;
export const MENU_SURFACE_VERTICAL_PADDING_PX = 8;
export const MENU_SEPARATOR_CLASS_NAME = "mx-1 my-1 border-t border-(--divider-subtle-color)";

/** 行、分隔线及条目间距必须和同一 flex 菜单列表的渲染高度一致。 */
export function getMenuContentHeight(itemHeights: readonly number[], separatorCount = 0): number {
  return MENU_SURFACE_VERTICAL_PADDING_PX
    + itemHeights.reduce((total, height) => total + height, 0)
    + separatorCount * 9
    + Math.max(0, itemHeights.length + separatorCount - 1) * MENU_ITEM_GAP_PX;
}

export const MENU_ITEM_BASE_CLASS_NAME =
  "w-full radius-control-lg text-left transition-[background-color,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]";

export function getMenuItemStateClassName({
  active = false,
  tone = "default",
}: {
  active?: boolean;
  tone?: UiMenuItemTone;
}): string {
  if (tone === "danger") {
    return active
      ? "bg-(--surface-interactive-active-background) text-(--destructive)"
      : "text-(--destructive) [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)]";
  }
  if (tone === "primary") {
    return active
      ? "bg-(--surface-interactive-active-background) text-(--brand-action)"
      : "text-(--brand-action) [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background)";
  }
  return active
    ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
    : "text-(--text-default) [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background) [&:not(:disabled):hover]:text-(--text-strong)";
}
