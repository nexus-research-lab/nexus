// INPUT: 菜单条目的 active 状态与 default/primary/danger tone。
// OUTPUT: Action、Select 与上下文菜单共用的间距、圆角和状态样式。
// POS: Menu 视觉合同；不渲染 DOM、定位浮层或持有业务选值。

type UiMenuItemTone = "default" | "primary" | "danger";

/** 菜单型浮层统一使用 4px 外边距和 2px 条目节奏。 */
export const MENU_LIST_CLASS_NAME = "flex flex-col gap-0.5";
export const MENU_ITEM_GAP_PX = 2;
export const MENU_SURFACE_VERTICAL_PADDING_PX = 8;

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
    return "text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)]";
  }
  if (tone === "primary") {
    return active
      ? "bg-(--surface-interactive-active-background) font-semibold text-(--brand-action)"
      : "text-(--brand-action) hover:bg-(--surface-interactive-hover-background)";
  }
  return active
    ? "bg-(--surface-interactive-active-background) font-semibold text-(--text-strong)"
    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)";
}
