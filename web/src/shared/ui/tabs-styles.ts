import { cn } from "@/lib/utils";

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
    density === "compact" ? "h-8 text-[10.5px]" : "h-9 text-[11px]",
    active
      ? "border-(--surface-interactive-active-border) text-(--text-strong)"
      : "text-(--text-default) hover:text-(--text-strong)",
    className,
  );
}
