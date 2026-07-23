import { cn } from "@/shared/ui/class-name";

interface UiListRowPresentation {
  className: string;
  role: "button" | undefined;
  tabIndex: 0 | undefined;
}

const LIST_ROW_STATE_CLASS_NAMES = {
  active: "bg-(--surface-interactive-active-background) text-(--text-strong)",
  idleDefault: "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  idleMuted: "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
} as const;

export function getUiListRowPresentation({
  active,
  className,
  inactiveTone,
  interactive,
}: {
  active: boolean;
  className?: string;
  inactiveTone: "default" | "muted";
  interactive: boolean;
}): UiListRowPresentation {
  const state = active
    ? "active"
    : inactiveTone === "muted"
      ? "idleMuted"
      : "idleDefault";
  return {
    className: cn(
      "group/item relative flex min-h-[64px] w-full items-center gap-3 rounded-[8px] px-2.5 py-2 text-left transition-[background,color] duration-(--motion-duration-fast)",
      interactive && "cursor-pointer",
      LIST_ROW_STATE_CLASS_NAMES[state],
      className,
    ),
    role: interactive ? "button" : undefined,
    tabIndex: interactive ? 0 : undefined,
  };
}
