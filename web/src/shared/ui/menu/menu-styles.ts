type UiMenuItemTone = "default" | "primary" | "danger";

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
