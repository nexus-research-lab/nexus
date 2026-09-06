// INPUT: 菜单条目密度、说明存在性、active 状态与 default/primary/danger tone。
// OUTPUT: Action、Mention 与上下文菜单共用的行尺寸/估算高度、圆角和状态样式。
// POS: Menu 视觉合同；不渲染 DOM、定位浮层或持有业务选值。

export type UiMenuItemTone = "default" | "primary" | "danger";
export type UiMenuItemDensity = "compact" | "default";

const MENU_ITEM_LAYOUT = {
  compact: {
    described: { className: "h-10 gap-2 px-2 py-0.5 text-compact", height: 40 },
    plain: { className: "h-8 gap-2 px-2 text-compact", height: 32 },
  },
  default: {
    described: { className: "h-11 gap-3 px-2.5 py-1 text-sm", height: 44 },
    plain: { className: "h-9 gap-3 px-2.5 text-sm", height: 36 },
  },
} as const;

/** Action rows and text suggestions share the same rendered and estimated geometry. */
export function getMenuItemLayout({ density = "default", hasDescription = false }: {
  density?: UiMenuItemDensity;
  hasDescription?: boolean;
} = {}) {
  return MENU_ITEM_LAYOUT[density][hasDescription ? "described" : "plain"];
}

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
