/**
 * INPUT: Launcher 标签与侧栏展开状态。
 * OUTPUT: 固定字号与字距、靠左的 Launcher 品牌字标，不随侧栏拉伸。
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
      <span
        className="sidebar-brand-wordmark relative w-max max-w-full cursor-pointer whitespace-nowrap leading-none"
        style={{
          fontFamily: '"Panchang", var(--font-sans)',
          fontSize: "22px",
          fontWeight: 320,
        }}
      >
        <span
          aria-hidden="true"
          className="absolute inset-x-0 top-0 flex gap-[0.16em] translate-y-[1.5px] text-[color:color-mix(in_srgb,var(--text-strong)_38%,transparent)] opacity-60 blur-[0.2px]"
        >
          {Array.from(wordmark, (letter, index) => <span key={index}>{letter}</span>)}
        </span>
        <span
          className="relative flex gap-[0.16em] bg-clip-text text-transparent transition-opacity duration-(--motion-duration-fast) group-hover/brand:opacity-80"
          style={{
            backgroundImage:
              "linear-gradient(180deg, color-mix(in srgb, var(--text-strong) 94%, white 6%) 4%, var(--text-default) 48%, color-mix(in srgb, var(--text-muted) 72%, var(--text-strong) 28%) 100%)",
            filter:
              "drop-shadow(0 1px 0 color-mix(in srgb, white 38%, transparent)) drop-shadow(0 4px 6px color-mix(in srgb, var(--text-strong) 12%, transparent))",
            WebkitBackgroundClip: "text",
          }}
        >
          {Array.from(wordmark, (letter, index) => <span key={index}>{letter}</span>)}
        </span>
      </span>
    </Link>
  );
}
