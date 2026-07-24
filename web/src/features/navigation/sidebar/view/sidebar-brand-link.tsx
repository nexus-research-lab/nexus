/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 指向 Launcher 的 NEXUS 品牌字标。
 * POS: 宽侧栏顶部唯一品牌入口，不承载 Agent 会话动作。
 */
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";

export function SidebarBrandLink({
  label,
}: {
  label: string;
}) {
  const wordmark = "NEXUS";
  return (
    <Link
      aria-label={label}
      className="group/brand relative isolate flex h-10 min-w-0 flex-1 items-center overflow-hidden"
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
      title={label}
      to={AppRouteBuilders.launcher()}
    >
      <span
        className="relative whitespace-nowrap text-lg uppercase leading-none tracking-[0.34em]"
        style={{
          fontFamily: '"Panchang", var(--font-sans)',
          fontWeight: 420,
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
