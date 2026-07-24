import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

interface SidebarNexusButtonProps {
  active: boolean;
  avatarSrc: string | null;
  label: string;
  onClick: () => void;
  variant: "dock" | "focus" | "rail";
}

export function SidebarNexusButton(props: SidebarNexusButtonProps) {
  const dock = props.variant === "dock";
  const focus = props.variant === "focus";
  const rail = props.variant === "rail";
  return (
    <button
      aria-current={props.active ? "page" : undefined}
      aria-label={rail ? props.label : undefined}
      className={cn(
        "relative flex items-center text-(--text-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
        dock
          ? "h-[54px] w-[52px] flex-col justify-center gap-1 rounded-[10px] text-2xs font-medium"
          : focus
            ? "h-10 w-[192px] justify-start gap-2 rounded-[10px] px-3 text-compact font-medium"
            : "h-9 w-9 justify-center rounded-full",
        props.active &&
          cn(SIDEBAR_SELECTION_CLASS_NAME, "text-(--text-strong)"),
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.nexus_agent}
      onClick={props.onClick}
      title={props.label}
      type="button"
    >
      <span
        className={cn(
          "flex items-center justify-center overflow-hidden rounded-[8px] border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[7px] font-semibold uppercase tracking-[0.08em] text-(--text-soft) shadow-(--surface-avatar-shadow)",
          rail ? "h-7 w-7" : "h-5 w-5",
          props.active &&
            "border-(--surface-interactive-active-border)",
        )}
      >
        {props.avatarSrc ? (
          <img
            alt=""
            className="h-full w-full object-cover"
            src={props.avatarSrc}
          />
        ) : (
          "NX"
        )}
      </span>
      {rail ? null : (
        <span
          className={cn(
            "max-w-full truncate leading-none",
            dock && "px-1",
          )}
        >
          {props.label}
        </span>
      )}
    </button>
  );
}
