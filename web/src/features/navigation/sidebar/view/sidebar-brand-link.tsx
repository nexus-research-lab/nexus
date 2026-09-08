/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 指向 Launcher 的 NEXUS 品牌字标。
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
  const wordmark = "NEXUS";
  return (
    <Link
      aria-label={label}
      aria-hidden={collapsed || undefined}
      className={cn(
        "group/brand relative isolate flex h-10 cursor-default items-center overflow-hidden transition-opacity duration-(--motion-duration-fast)",
        collapsed
          ? "min-w-0 flex-1 pointer-events-none opacity-0"
          : "shrink-0",
      )}
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
      tabIndex={collapsed ? -1 : undefined}
      to={AppRouteBuilders.launcher()}
    >
      <span
        className="sidebar-brand-wordmark relative cursor-pointer whitespace-nowrap uppercase leading-none"
        style={{
          fontFamily: '"Panchang", var(--font-sans)',
          fontSize: "var(--sidebar-brand-font-size, var(--text-lg))",
          fontWeight: 280,
          letterSpacing:
            "var(--sidebar-brand-letter-spacing, 0.98em)",
        }}
      >
        <span
          aria-hidden="true"
          className="absolute inset-x-0 top-0 translate-y-[1.5px] text-[color:color-mix(in_srgb,var(--text-strong)_38%,transparent)] opacity-60 blur-[0.2px]"
        >
          {wordmark}
        </span>
        <span
          className="relative bg-clip-text text-transparent transition-opacity duration-(--motion-duration-fast) group-hover/brand:opacity-80"
          style={{
            backgroundImage:
              "linear-gradient(180deg, color-mix(in srgb, var(--text-strong) 94%, white 6%) 4%, var(--text-default) 48%, color-mix(in srgb, var(--text-muted) 72%, var(--text-strong) 28%) 100%)",
            filter:
              "drop-shadow(0 1px 0 color-mix(in srgb, white 38%, transparent)) drop-shadow(0 4px 6px color-mix(in srgb, var(--text-strong) 12%, transparent))",
            WebkitBackgroundClip: "text",
          }}
        >
          {wordmark}
        </span>
      </span>
    </Link>
  );
}
