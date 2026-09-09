/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 固定尺寸、纯色紧凑的 Nexus 字标，不随侧栏拉伸。
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
  const wordmark = "Nexus";
  return (
    <Link
      aria-label={label}
      aria-hidden={collapsed || undefined}
      className={cn(
        "group/brand relative isolate flex h-10 min-w-0 flex-1 cursor-default items-center overflow-hidden transition-opacity duration-(--motion-duration-fast)",
        collapsed
          ? "min-w-0 flex-1 pointer-events-none opacity-0"
          : "",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
      tabIndex={collapsed ? -1 : undefined}
      to={AppRouteBuilders.launcher()}
    >
      <span className="sidebar-brand-wordmark cursor-pointer whitespace-nowrap font-sans text-[24px] leading-none font-semibold tracking-[-0.035em] text-(--text-strong)">
        {wordmark}
      </span>
    </Link>
  );
}
