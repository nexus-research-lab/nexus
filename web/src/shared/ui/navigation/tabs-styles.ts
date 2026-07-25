import { cn } from "@/shared/ui/class-name";

export type UiTabsDensity = "default" | "compact";

interface UiUnderlineTabStyleOptions {
  active?: boolean;
  density?: UiTabsDensity;
}

export function getUiUnderlineTabsNavClassName(className?: string): string {
  return cn(
    "soft-scrollbar scrollbar-hide flex min-w-0 items-center gap-4 overflow-x-auto",
    className,
  );
}

export function getUiTabDismissClassName(className?: string): string {
  return cn(
    "flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs text-(--icon-muted) transition-[background-color,color,opacity] duration-(--motion-duration-fast) hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive) hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_32%,transparent)]",
    className,
  );
}

export function getUiUnderlineTabClassName(
  options: UiUnderlineTabStyleOptions = {},
  className?: string,
): string {
  const {
    active = false,
    density = "default",
  } = options;

  return cn(
    "inline-flex shrink-0 items-center gap-1.5 border-b-2 border-transparent px-0 py-0 font-semibold transition-[color,border-color] duration-(--motion-duration-fast) ease-out",
    density === "compact" ? "h-8 text-xs" : "h-9 text-xs",
    active
      ? "border-[color:color-mix(in_srgb,var(--text-strong)_72%,transparent)] text-(--text-strong)"
      : "text-(--text-default) hover:text-(--text-strong)",
    className,
  );
}
