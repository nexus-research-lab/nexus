import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { GlassMagnifierStatic } from "@/shared/ui/liquid-glass";
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
        "flex h-9 w-9 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[10px] font-semibold uppercase text-(--text-subtle) shadow-(--surface-avatar-shadow) transition-(transform,border-color,box-shadow) duration-(--motion-duration-fast) hover:-translate-y-px hover:border-(--surface-interactive-hover-border)",
        active &&
          "border-(--surface-interactive-active-border) shadow-[0_8px_20px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
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
  prefersReducedMotion,
}: SidebarNexusButtonProps) {
  return (
    <button
      className="group/nexus relative flex h-10 w-[46px] shrink-0 items-center justify-center"
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.nexus_agent}
      onClick={onClick}
      title="Nexus"
      type="button"
    >
      <GlassMagnifierStatic
        className={cn(
          "relative z-10 transition-transform duration-(--motion-duration-normal)",
          !prefersReducedMotion && "group-hover/nexus:scale-[1.03]",
          active &&
            "drop-shadow-[0_8px_20px_color-mix(in_srgb,var(--primary)_12%,transparent)]",
        )}
        height={34}
        underlay={
          active ? (
            <NexusActiveUnderlay prefersReducedMotion={prefersReducedMotion} />
          ) : undefined
        }
        width={46}
      >
        <span className="relative flex h-7 w-7 items-center justify-center">
          <span
            className={cn(
              "relative z-10 flex h-7 w-7 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
              active &&
                "shadow-[0_0_0_1px_rgba(255,255,255,0.14),0_0_10px_color-mix(in_srgb,var(--primary)_8%,transparent)]",
            )}
          >
            {active ? <NexusGlassHighlight /> : null}
            {avatarSrc ? (
              <img
                alt="Nexus"
                className="relative z-10 h-full w-full object-cover"
                src={avatarSrc}
              />
            ) : (
              <span className="relative z-10 text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-subtle)">
                NX
              </span>
            )}
          </span>
        </span>
      </GlassMagnifierStatic>
    </button>
  );
}

function NexusActiveUnderlay({
  prefersReducedMotion,
}: {
  prefersReducedMotion: boolean;
}) {
  return (
    <>
      {/* 彩光位于玻璃下层，让折射和高光基于真实下层内容。 */}
      <span
        className={cn(
          "absolute left-1/2 top-1/2 h-[36px] w-[36px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-88 blur-[0.5px]",
          !prefersReducedMotion && "animate-[spin_5.2s_linear_infinite]",
        )}
        style={{
          background:
            "conic-gradient(from 180deg, transparent 0deg, transparent 24deg, rgba(96,165,250,0.98) 58deg, rgba(167,139,250,0.92) 104deg, transparent 146deg, transparent 206deg, rgba(52,211,153,0.9) 240deg, rgba(245,158,11,0.92) 280deg, rgba(244,114,182,0.94) 320deg, transparent 348deg, transparent 360deg)",
          mask: "radial-gradient(farthest-side, transparent calc(100% - 3px), #000 calc(100% - 1px))",
          WebkitMask:
            "radial-gradient(farthest-side, transparent calc(100% - 3px), #000 calc(100% - 1px))",
        }}
      />
      <span
        className={cn(
          "absolute left-1/2 top-1/2 h-[28px] w-[28px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-48 blur-[8px]",
          !prefersReducedMotion &&
            "animate-[spin_8.6s_linear_infinite_reverse]",
        )}
        style={{
          background:
            "conic-gradient(from 180deg, transparent 0deg, rgba(96,165,250,0.84) 66deg, transparent 136deg, transparent 214deg, rgba(244,114,182,0.82) 292deg, rgba(52,211,153,0.74) 336deg, transparent 360deg)",
        }}
      />
      <span className="absolute left-1/2 top-1/2 h-[24px] w-[24px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[radial-gradient(circle_at_34%_28%,rgba(255,255,255,0.34),transparent_42%),radial-gradient(circle_at_68%_72%,rgba(255,255,255,0.14),transparent_48%)] opacity-82 blur-[3px]" />
    </>
  );
}

function NexusGlassHighlight() {
  return (
    <>
      {/* 这一层只保留轻微反光，主动态来自玻璃下层的彩光。 */}
      <span className="pointer-events-none absolute inset-0 z-20 rounded-full bg-[radial-gradient(circle_at_28%_24%,rgba(255,255,255,0.24),transparent_38%),linear-gradient(132deg,rgba(255,255,255,0.18),transparent_42%,transparent_60%,rgba(255,255,255,0.08))] mix-blend-screen opacity-72" />
      <span className="pointer-events-none absolute inset-[1px] z-20 rounded-full border border-[rgba(255,255,255,0.22)] opacity-72" />
    </>
  );
}
