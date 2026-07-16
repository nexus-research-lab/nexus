import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { cn } from "@/shared/ui/class-name";

interface SidebarNexusButtonProps {
  active: boolean;
  avatarSrc: string | null;
  onClick: () => void;
  prefersReducedMotion: boolean;
  variant: "rail" | "panel";
}

export function SidebarNexusButton(props: SidebarNexusButtonProps) {
  return props.variant === "rail" ? (
    <RailNexusButton {...props} />
  ) : (
    <PanelNexusButton {...props} />
  );
}

function RailNexusButton({
  active,
  avatarSrc,
  onClick,
}: SidebarNexusButtonProps) {
  return (
    <button
      aria-label="Nexus"
      className={cn(
        "flex h-9 w-9 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[10px] font-semibold uppercase text-(--text-subtle) shadow-(--surface-avatar-shadow) transition-[border-color,background-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border)",
        active &&
          "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background)",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.nexus_agent}
      onClick={onClick}
      title="Nexus"
      type="button"
    >
      {avatarSrc ? (
        <img alt="" className="h-full w-full object-cover" src={avatarSrc} />
      ) : (
        "NX"
      )}
    </button>
  );
}

function PanelNexusButton({
  active,
  avatarSrc,
  onClick,
}: SidebarNexusButtonProps) {
  return (
    <button
      className={cn(
        "group/nexus relative flex h-10 w-[46px] shrink-0 items-center justify-center",
        "rounded-[8px] transition-colors duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background)",
        active && "bg-(--surface-interactive-active-background)",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.nexus_agent}
      onClick={onClick}
      title="Nexus"
      type="button"
    >
      <span
        className={cn(
          "flex h-8 w-8 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
          active && "border-(--surface-interactive-active-border)",
        )}
      >
        {avatarSrc ? (
          <img alt="Nexus" className="h-full w-full object-cover" src={avatarSrc} />
        ) : (
          <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-subtle)">
            NX
          </span>
        )}
      </span>
    </button>
  );
}
