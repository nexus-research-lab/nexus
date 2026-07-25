import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

interface UiListRowPresentation {
  className: string;
  role: "button" | undefined;
  tabIndex: 0 | undefined;
}

const LIST_ROW_STATE_CLASS_NAMES = {
  active: "border-transparent bg-(--surface-interactive-active-background) text-(--text-strong) shadow-none",
  activeSidebar: cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
  idleDefault: "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  idleMuted: "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
} as const;

export function getUiListRowPresentation({
  active,
  activeTone,
  className,
  inactiveTone,
  interactive,
}: {
  active: boolean;
  activeTone: "default" | "sidebar";
  className?: string;
  inactiveTone: "default" | "muted";
  interactive: boolean;
}): UiListRowPresentation {
  const state = active
    ? activeTone === "sidebar" ? "activeSidebar" : "active"
    : inactiveTone === "muted"
      ? "idleMuted"
      : "idleDefault";
  return {
    className: cn(
      "group/item relative flex min-h-[64px] w-full items-center gap-3 rounded-[10px] border border-transparent px-2.5 py-2 text-left transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast)",
      interactive && "cursor-pointer",
      LIST_ROW_STATE_CLASS_NAMES[state],
      className,
    ),
    role: interactive ? "button" : undefined,
    tabIndex: interactive ? 0 : undefined,
  };
}
