/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 使用 Panchang 字体与舒展字距的纯色 Launcher 品牌字标与沿玻璃轮廓流动的圆角矩形彩光。
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
    <span
      className="absolute inset-0 rounded-[inherit] p-[2px] opacity-88 blur-[0.5px]"
      style={{
        mask: "linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0)",
        maskComposite: "exclude",
        WebkitMask: "linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0)",
        WebkitMaskComposite: "xor",
      }}
    >
      {/* 固定镂空轮廓贴合玻璃盖板，仅旋转下层彩色纹理。 */}
      <span
        className={cn(
          "absolute left-1/2 top-1/2 aspect-square w-[150%] -translate-x-1/2 -translate-y-1/2",
          "motion-safe:animate-[spin_5.2s_linear_infinite_paused] group-hover/glass:[animation-play-state:running]",
        )}
        style={{
          background:
            "conic-gradient(from 180deg, transparent 0deg, transparent 24deg, rgba(96,165,250,0.98) 58deg, rgba(167,139,250,0.92) 104deg, transparent 146deg, transparent 206deg, rgba(52,211,153,0.9) 240deg, rgba(245,158,11,0.92) 280deg, rgba(244,114,182,0.94) 320deg, transparent 348deg, transparent 360deg)",
        }}
      />
    </span>
  );
}
