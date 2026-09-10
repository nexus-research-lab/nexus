/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 使用 Panchang 字体与舒展字距的纯色 Launcher 品牌字标。
 * POS: 宽侧栏顶部唯一品牌入口，不承载 Agent 会话动作。
 */
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
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
        "group/brand relative flex h-10 min-w-0 flex-1 cursor-default items-center overflow-hidden transition-opacity duration-(--motion-duration-fast)",
        collapsed
          ? "min-w-0 flex-1 pointer-events-none opacity-0"
          : "",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
      tabIndex={collapsed ? -1 : undefined}
      to={AppRouteBuilders.launcher()}
    >
      <span
        className="sidebar-brand-wordmark cursor-pointer whitespace-nowrap text-[18px] font-[280] leading-none tracking-[0.18em] text-(--text-strong) transition-opacity duration-(--motion-duration-fast) group-hover/brand:opacity-80"
        style={{ fontFamily: '"Panchang", var(--font-sans)' }}
      >
        Nexus
      </span>
    </Link>
  );
}
