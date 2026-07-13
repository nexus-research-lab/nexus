import { cn } from "@/shared/ui/class-name";

interface UiListRowPresentation {
  className: string;
  role: "button" | undefined;
  showActiveIndicator: boolean;
  tabIndex: 0 | undefined;
}

const LIST_ROW_STATE_CLASS_NAMES = {
  active: "bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-elevated-background))] text-(--text-strong) shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_12%,transparent)]",
  idle: "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
} as const;

export function getUiListRowPresentation({
  active,
  className,
  interactive,
}: {
  active: boolean;
  className?: string;
  interactive: boolean;
}): UiListRowPresentation {
  const state = active ? "active" : "idle";
  return {
    className: cn(
      "group/item relative flex min-h-[68px] w-full items-center gap-3 rounded-[14px] px-3 py-2.5 text-left transition-[background,color,transform] duration-(--motion-duration-fast)",
      interactive && "cursor-pointer",
      LIST_ROW_STATE_CLASS_NAMES[state],
      className,
    ),
    role: interactive ? "button" : undefined,
    showActiveIndicator: active,
    tabIndex: interactive ? 0 : undefined,
  };
}
