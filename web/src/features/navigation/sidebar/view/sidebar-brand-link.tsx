/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 使用 Panchang 字体与舒展字距的纯色 Launcher 品牌字标。
 * POS: 宽侧栏顶部唯一品牌入口，不承载 Agent 会话动作。
 */
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { GlassMagnifier } from "@/shared/ui/liquid-glass/glass-magnifier";
import { cn } from "@/shared/ui/class-name";

export function SidebarBrandLink({
  collapsed,
  label,
}: {
  collapsed: boolean;
  label: string;
}) {
  return (
    <Link
      aria-label={label}
      aria-hidden={collapsed || undefined}
      className={cn(
        "group/brand @container relative flex h-10 min-w-0 flex-1 cursor-default items-center overflow-hidden transition-opacity duration-(--motion-duration-fast)",
        collapsed
          ? "min-w-0 flex-1 pointer-events-none opacity-0"
          : "",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
      tabIndex={collapsed ? -1 : undefined}
      to={AppRouteBuilders.launcher()}
    >
      <GlassMagnifier underlay={<NexusGlassUnderlay />}>
        <span
          className="sidebar-brand-wordmark cursor-pointer whitespace-nowrap text-[clamp(12px,calc((100cqw_-_24px)/7),18px)] font-[400] leading-none tracking-[0.18em] text-(--text-strong) transition-opacity duration-(--motion-duration-fast) group-hover/brand:opacity-80"
          style={{ fontFamily: '"Panchang", var(--font-sans)' }}
        >
          NEXUS
        </span>
      </GlassMagnifier>
    </Link>
  );
}

function NexusGlassUnderlay() {
  return (
    <>
      {/* 彩光位于玻璃下层，让折射和高光基于真实下层内容。 */}
      <span
        className={cn(
          "absolute left-1/2 top-1/2 h-[36px] w-[36px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-88 blur-[0.5px]",
          "motion-safe:animate-[spin_5.2s_linear_infinite_paused] group-hover/glass:[animation-play-state:running]",
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
          "motion-safe:animate-[spin_8.6s_linear_infinite_reverse_paused] group-hover/glass:[animation-play-state:running]",
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
